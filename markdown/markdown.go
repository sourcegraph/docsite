package markdown

import (
	"bytes"
	"context"
	"fmt"
	gohtml "html"
	"io"
	"net/url"
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/Depado/bfchroma"
	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/styles"
	"github.com/pkg/errors"
	"github.com/russross/blackfriday/v2"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// Document is a parsed and HTML-rendered Markdown document.
type Document struct {
	// Meta is the document's metadata in the Markdown "front matter", if any.
	Meta Metadata

	// Title is taken from the metadata (if it exists) or else from the text content of the first
	// heading.
	Title string

	// HTML is the rendered Markdown content.
	HTML []byte

	// Tree is the tree of sections (used to show a table of contents).
	Tree []*SectionNode
}

// Options customize how Run parses and HTML-renders the Markdown document.
type Options struct {
	// Base is the base URL (typically including only the path, such as "/" or "/help/") to use when
	// resolving relative links.
	Base *url.URL

	// ContentFilePathToLinkPath converts references to file paths of other content files to the URL
	// path to use in links. For example, ContentFilePathToLinkPath("a/index.md") == "a".
	ContentFilePathToLinkPath func(string) string

	// Funcs are custom functions that can be invoked within Markdown documents with
	// <markdownfuncjfunction-name arg="val" />.
	Funcs FuncMap

	// FuncInfo contains information passed to Markdown functions about the current execution
	// context.
	FuncInfo FuncInfo
}

// FuncMap contains named functions that can be invoked within Markdown documents (see
// (Options).Funcs).
type FuncMap map[string]func(context.Context, FuncInfo, map[string]string) (string, error)

// FuncInfo contains information passed to Markdown functions about the current execution context.
type FuncInfo struct {
	Version string // the version of the content containing the page to render
}

// NewParser creates a new Markdown parser (the same one used by Run).
func NewParser(renderer blackfriday.Renderer) *blackfriday.Markdown {
	return blackfriday.New(
		blackfriday.WithRenderer(renderer),
		blackfriday.WithExtensions(blackfriday.CommonExtensions),
	)
}

// styles.VisualStudio without bg:#ffffff.
var chromaStyle = styles.Register(chroma.MustNewStyle("vs", chroma.StyleEntries{
	chroma.Comment:           "#008000",
	chroma.CommentPreproc:    "#0000ff",
	chroma.Keyword:           "#0000ff",
	chroma.OperatorWord:      "#0000ff",
	chroma.KeywordType:       "#2b91af",
	chroma.NameClass:         "#2b91af",
	chroma.LiteralString:     "#a31515",
	chroma.GenericHeading:    "bold",
	chroma.GenericSubheading: "bold",
	chroma.GenericEmph:       "italic",
	chroma.GenericStrong:     "bold",
	chroma.GenericPrompt:     "bold",
	chroma.Error:             "border:#FF0000",
}))

// NewBfRenderer creates the default blackfriday render to be passed to NewParser()
func NewBfRenderer() blackfriday.Renderer {
	return bfchroma.NewRenderer(
		bfchroma.ChromaStyle(chromaStyle),
		bfchroma.Extend(
			blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
				Flags: blackfriday.CommonHTMLFlags,
			}),
		),
	)
}

func New(opt Options) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			&extender{Options: opt},
			extension.GFM,
			extension.DefinitionList,
			extension.Typographer,
			highlighting.Highlighting,
		),
	)
}

// Run parses and HTML-renders a Markdown document (with optional metadata in the Markdown "front
// matter").
func Run(ctx context.Context, input []byte, opt Options) (doc *Document, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.Errorf("panic while rendering Markdown: %s", e)
		}
	}()

	meta, source, err := parseMetadata(input)
	if err != nil {
		return nil, err
	}

	md := New(opt)
	var buf bytes.Buffer
	err = md.Convert(source, &buf)
	if err != nil {
		return nil, err
	}

	// TODO: Use renderer.NodeRenderer to collect tree and title without parsing the second time.
	ast := md.Parser().Parse(text.NewReader(source))
	doc = &Document{
		Meta: meta,
		HTML: buf.Bytes(),
		Tree: newTree(ast, source),
	}
	if meta.Title != "" {
		doc.Title = meta.Title
	} else {
		doc.Title = GetTitle(ast, source)
	}

	return doc, nil
}

type bfRenderer struct {
	Options
	blackfriday.Renderer

	errors []error
}

func (r *bfRenderer) RenderNode(ctx context.Context, w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	switch node.Type {
	case blackfriday.Link, blackfriday.Image:
		// Bypass the (HTMLRendererParams).AbsolutePrefix field entirely and perform our own URL
		// resolving. This fixes the issue reported in
		// https://github.com/russross/blackfriday.v2/pull/231 where relative URLs starting with "."
		// are not treated as relative URLs.
		if entering {
			dest, err := url.Parse(string(node.LinkData.Destination))
			if err == nil && !dest.IsAbs() && dest.Path != "" {
				if r.Options.ContentFilePathToLinkPath != nil {
					dest.Path = r.Options.ContentFilePathToLinkPath(dest.Path)
				}
				if r.Options.Base != nil {
					dest = r.Options.Base.ResolveReference(dest)
				}
				node.LinkData.Destination = []byte(dest.String())
			}
		}
	case blackfriday.HTMLBlock, blackfriday.HTMLSpan:
		// Rewrite URLs correctly when they are relative to the document, regardless of whether it's
		// an index.md document or not.
		if entering && r.Options.Base != nil {
			if v, err := rewriteRelativeURLsInHTML(node.Literal, r.Options); err == nil {
				node.Literal = v
			}
		}
		// Evaluate Markdown funcs (<div markdown-func=name ...> nodes), using a heuristic to
		// skip blocks that don't contain any invocations.
		if entering {
			if v, err := EvalMarkdownFuncs(ctx, node.Literal, r.Options); err == nil {
				node.Literal = v
			} else {
				r.errors = append(r.errors, err)
			}
		}

	case blackfriday.BlockQuote:
		parseAside := func(literal []byte) string {
			switch {
			case bytes.HasPrefix(literal, []byte("NOTE:")):
				return "note"
			case bytes.HasPrefix(literal, []byte("WARNING:")):
				return "warning"
			default:
				return ""
			}
		}
		if node.FirstChild.FirstChild != nil {
			if asideClass := parseAside(node.FirstChild.FirstChild.Literal); asideClass != "" {
				if entering {
					fmt.Fprintf(w, "<aside class=\"%s\">\n", asideClass)
				} else {
					fmt.Fprint(w, "</aside>\n")
				}
				return blackfriday.GoToNext
			}
		}
	case blackfriday.Text:
		if entering {
			if newNodes := rewriteAnchorDirectives(node); len(newNodes) > 0 {
				for _, n := range newNodes {
					if status := r.Renderer.RenderNode(w, n, entering); status != blackfriday.GoToNext {
						return status
					}
				}
				return blackfriday.GoToNext
			}
		}
	}
	return r.Renderer.RenderNode(w, node, entering)
}

var anchorDirectivePattern = regexp.MustCompile(`\{#[\w.-]+\}`)

// rewriteAnchorDirectives rewrites `{#foo}` directives in text to `<a id="foo"></a>` anchors.
func rewriteAnchorDirectives(node *blackfriday.Node) []*blackfriday.Node {
	matches := anchorDirectivePattern.FindAllIndex(node.Literal, -1)
	if len(matches) == 0 {
		return nil
	}

	out := make([]*blackfriday.Node, 0, 2*len(matches)+1)
	appendTextNode := func(text []byte) {
		n := blackfriday.NewNode(blackfriday.Text)
		n.Literal = text
		out = append(out, n)
	}

	i := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		if i != start {
			appendTextNode(node.Literal[i:start])
		}

		n := blackfriday.NewNode(blackfriday.HTMLSpan)
		escapedID := gohtml.EscapeString(string(node.Literal[start+2 : end-1]))
		n.Literal = []byte(fmt.Sprintf(`<span id="%s" class="anchor-inline"></span><a href="#%s" class="anchor-inline-link" title="#%s"></a>`, escapedID, escapedID, escapedID))
		out = append(out, n)

		i = end
	}
	if i != len(node.Literal) {
		appendTextNode(node.Literal[i:len(node.Literal)])
	}
	return out
}

// IsDocumentTopTitleHeadingNode reports whether node is an h1-heading at the top of the document.
func IsDocumentTopTitleHeadingNode(node *blackfriday.Node) bool {
	if node.Parent != nil && node.Parent.Type == blackfriday.Document {
		doc := node.Parent
		return getDocumentTopTitleHeadingNode(doc) == node
	}
	return false
}

func getDocumentTopTitleHeadingNode(doc *blackfriday.Node) *blackfriday.Node {
	if doc.Type != blackfriday.Document {
		panic(fmt.Sprintf("got node type %q, want %q", doc.Type, blackfriday.Document))
	}

	for node := doc.FirstChild; node != nil; node = node.Next {
		if node.Type == blackfriday.HTMLBlock && isOnlyHTMLComment(node.Literal) {
			continue
		}
		if node.Type == blackfriday.Heading && node.HeadingData.Level == 1 {
			return node
		}
		return nil
	}
	return nil
}

func GetTitleOld(doc *blackfriday.Node) string {
	title := getDocumentTopTitleHeadingNode(doc)
	if title != nil {
		return string(RenderTextOld(title))
	}
	return ""
}

func GetTitle(doc ast.Node, source []byte) string {
	if doc.Kind() != ast.KindDocument {
		panic(fmt.Sprintf("got node type %q, want %q", doc.Kind(), ast.KindDocument))
	}

	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if node.Kind() != ast.KindHeading {
			continue
		}

		n := node.(*ast.Heading)
		if n.Level != 1 || n.Lines().Len() == 0 {
			break
		}

		return string(RenderText(n, source))
	}
	return ""
}

func RenderTextOld(node *blackfriday.Node) []byte {
	var parts [][]byte
	node.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if node.Type == blackfriday.Text || node.Type == blackfriday.Code {
			parts = append(parts, node.Literal)
		}
		return blackfriday.GoToNext
	})
	return bytes.TrimSpace(joinBytesAsText(parts))
}

func RenderText(node ast.Node, source []byte) []byte {
	parts := make([][]byte, node.Lines().Len())
	for i := range parts {
		s := node.Lines().At(i)
		parts[i] = (&s).Value(source)
	}
	return bytes.TrimSpace(joinBytesAsText(parts))
}

// joinBytesAsText joins parts, adding spaces between adjacent parts unless there is already space
// or a punctuation at the boundary.
func joinBytesAsText(parts [][]byte) []byte {
	// Preallocate buffer to maximum size needed.
	size := 0
	for _, part := range parts {
		size += len(part) + 1
	}
	buf := bytes.NewBuffer(make([]byte, 0, size))

	for i, part := range parts {
		if i != 0 {
			if r, size := utf8.DecodeRune(part); r != utf8.RuneError && size > 0 {
				if !unicode.IsPunct(r) {
					_ = buf.WriteByte(' ')
				}
			}
		}
		_, _ = buf.Write(part)
	}
	return buf.Bytes()
}

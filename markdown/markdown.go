package markdown

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Depado/bfchroma"
	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/pkg/errors"
	"github.com/russross/blackfriday/v2"
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

// NewBfRenderer creates the default blackfriday renderer to be passed to NewParser()
func NewBfRenderer() blackfriday.Renderer {
	return bfchroma.NewRenderer(
		bfchroma.ChromaOptions(chromahtml.WithClasses(true)),
		bfchroma.Extend(
			blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
				Flags: blackfriday.CommonHTMLFlags,
			}),
		),
	)
}

var pipePlaceholder = []byte("\xe2\xa6\x80")

// escapePipesInBackticks works around https://github.com/russross/blackfriday/issues/207,
// substituting unicode character U+2980 (triple vertical bar) for escaped pipe symbols "\|"
// occurring within backtick-delimited code blocks.
func escapePipesInBackticks(b []byte) ([]byte, error) {
	if bytes.Contains(b, pipePlaceholder) {
		return nil, errors.Errorf("unhandled case: placeholder %s is already in the document", pipePlaceholder)
	}
	in := false
	b2 := make([]byte, 0, len(b))
	i := 0
	for i < len(b) {
		switch {
		case b[i] == '`':
			in = !in
			b2 = append(b2, b[i])
		case b[i] == '\\' && i+1 < len(b) && b[i+1] == '|':
			if in {
				b2 = append(b2, pipePlaceholder...)
			}
			i++
		case b[i] == '\n':
			in = false
			b2 = append(b2, b[i])
		default:
			b2 = append(b2, b[i])
		}
		i++
	}
	return b2, nil
}

// unescapePipes reverses the escapes done in escapePipesInBackticks.
func unescapePipes(b []byte) []byte {
	return bytes.Replace(b, pipePlaceholder, []byte("|"), -1)
}

// Run parses and HTML-renders a Markdown document (with optional metadata in the Markdown "front
// matter").
func Run(ctx context.Context, input []byte, opt Options) (doc *Document, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.Errorf("panic while rendering Markdown: %s", e)
		}
	}()

	input, err = escapePipesInBackticks(input)
	if err != nil {
		return nil, errors.Wrap(err, "escaping pipes within backticks")
	}

	meta, markdown, err := parseMetadata(input)
	if err != nil {
		return nil, err
	}

	bfRenderer := NewBfRenderer()
	ast := NewParser(bfRenderer).Parse(markdown)
	renderer := &renderer{
		Options:  opt,
		Renderer: bfRenderer,
	}
	var buf bytes.Buffer
	SetHeadingIDs(ast)
	ast.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		return renderer.RenderNode(ctx, &buf, node, entering)
	})

	doc = &Document{
		Meta: meta,
		HTML: unescapePipes(buf.Bytes()),
		Tree: newTree(ast),
	}
	if meta.Title != "" {
		doc.Title = meta.Title
	} else {
		doc.Title = GetTitle(ast)
	}

	if len(renderer.errors) > 0 {
		err = renderer.errors[0]
	}
	return doc, err
}

type renderer struct {
	Options
	blackfriday.Renderer

	errors []error
}

func (r *renderer) RenderNode(ctx context.Context, w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	switch node.Type {
	case blackfriday.Heading:
		// Add "#" anchor links to headers to make it easy for users to discover and copy links
		// to sections of a document.
		if status := r.Renderer.RenderNode(w, node, entering); status != blackfriday.GoToNext {
			return status
		}
		if entering {
			// If heading consists only of a link, do not emit an anchor link.
			if hasSingleChildOfType(node, blackfriday.Link) {
				fmt.Fprintf(w, `<a name="%s" aria-hidden="true"></a>`, node.HeadingID)
			} else {
				fmt.Fprintf(w, `<a name="%s" class="anchor" href="#%s" rel="nofollow" aria-hidden="true" title="#%s"></a>`, node.HeadingID, node.HeadingID, node.HeadingID)
			}
		}
		return blackfriday.GoToNext
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

					// Remove trailing slashes, which are never used for content page links (except the root).
					if dest.Path != "/" {
						dest.Path = strings.TrimSuffix(dest.Path, "/")
					}
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
		escapedID := html.EscapeString(string(node.Literal[start+2 : end-1]))
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
		if node.Type == blackfriday.Heading && node.HeadingData.Level == 1 {
			return node
		}
	}
	return nil
}

func GetTitle(doc *blackfriday.Node) string {
	title := getDocumentTopTitleHeadingNode(doc)
	if title != nil {
		return string(RenderText(title))
	}
	return ""
}

func RenderText(node *blackfriday.Node) []byte {
	var parts [][]byte
	node.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if node.Type == blackfriday.Text || node.Type == blackfriday.Code {
			parts = append(parts, node.Literal)
		}
		return blackfriday.GoToNext
	})
	return bytes.TrimSpace(joinBytesAsText(parts))
}

// joinBytesAsText joins parts, adding spaces between adjacent parts unless there is already space
// or a punctiation at the boundary.
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

func hasSingleChildOfType(node *blackfriday.Node, typ blackfriday.NodeType) bool {
	seenLink := false
	for child := node.FirstChild; child != nil; child = child.Next {
		switch {
		case child.Type == blackfriday.Text && len(child.Literal) == 0:
			continue
		case child.Type == blackfriday.Link && !seenLink:
			seenLink = true
		default:
			return false
		}
	}
	return seenLink
}

func getFirstChildLink(node *blackfriday.Node) *blackfriday.Node {
	for child := node.FirstChild; child != nil; child = child.Next {
		if child.Type == blackfriday.Link {
			return child
		}
	}
	return nil
}

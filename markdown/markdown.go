package markdown

import (
	"bytes"
	"context"
	"fmt"
	gohtml "html"
	"io"
	"net/url"
	"regexp"

	chromahtml "github.com/alecthomas/chroma/formatters/html"
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
			highlighting.NewHighlighting(
				highlighting.WithStyle("vs"),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(false),
				),
			),
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

	// TODO: https://sourcegraph.com/github.com/gohugoio/hugo@dd31e800075eebd78f921df8b4865c238006e7a7/-/blob/markup/goldmark/toc.go#L56
	root := md.Parser().Parse(text.NewReader(source))
	tree, err := newTree(root, source)
	if err != nil {
		return nil, err
	}

	doc = &Document{
		Meta: meta,
		HTML: buf.Bytes(),
		Tree: tree,
	}
	if meta.Title != "" {
		doc.Title = meta.Title
	} else {
		doc.Title = GetTitle(root, source)
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
func IsDocumentTopTitleHeadingNode(node ast.Node) bool {
	if node.Parent() != nil && node.Parent().Kind() == ast.KindDocument {
		return getDocumentTopTitleHeadingNode(node.Parent()) == node
	}
	return false
}

func getDocumentTopTitleHeadingNode(doc ast.Node) ast.Node {
	if doc.Kind() != ast.KindDocument {
		panic(fmt.Sprintf("got node type %q, want %q", doc.Kind(), ast.KindDocument))
	}

	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if node.Kind() != ast.KindHeading {
			continue
		}

		n := node.(*ast.Heading)
		if n.Level == 1 {
			return n
		}
		return nil
	}
	return nil
}

func GetTitle(doc ast.Node, source []byte) string {
	if doc.Kind() != ast.KindDocument {
		panic(fmt.Sprintf("got node type %q, want %q", doc.Kind(), ast.KindDocument))
	}

	title := getDocumentTopTitleHeadingNode(doc)
	if title != nil {
		return string(title.Text(source))
	}

	return ""
}

package markdown

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/pkg/errors"

	"github.com/Depado/bfchroma"
	"github.com/alecthomas/chroma/styles"
	"github.com/shurcooL/sanitized_anchor_name"
	"gopkg.in/russross/blackfriday.v2"
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
		blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.AutoHeadingIDs),
	)
}

// NewBfRenderer creates the default blackfriday renderer to be passed to NewParser()
func NewBfRenderer() blackfriday.Renderer {
	return bfchroma.NewRenderer(
		bfchroma.ChromaStyle(styles.VisualStudio),
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
func Run(ctx context.Context, input []byte, opt Options) (*Document, error) {
	input, err := escapePipesInBackticks(input)
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
		Options:    opt,
		Renderer:   bfRenderer,
		headingIDs: map[string]int{},
	}
	var buf bytes.Buffer
	ast.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		return renderer.RenderNode(ctx, &buf, node, entering)
	})

	doc := Document{
		Meta: meta,
		HTML: unescapePipes(buf.Bytes()),
		Tree: newTree(ast),
	}
	if meta.Title != "" {
		doc.Title = meta.Title
	} else {
		doc.Title = getTitle(ast)
	}

	if len(renderer.errors) > 0 {
		err = renderer.errors[0]
	}
	return &doc, err
}

type renderer struct {
	Options
	blackfriday.Renderer

	errors []error

	headingIDs map[string]int // for generating unique heading IDs
}

func (r *renderer) RenderNode(ctx context.Context, w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	switch node.Type {
	case blackfriday.Heading:
		if entering {
			// Make the heading ID based on the text contents, not the raw contents.
			node.HeadingID = sanitized_anchor_name.Create(renderText(node))

			// Ensure the heading ID is unique. The blackfriday package (in ensureUniqueHeadingID)
			// also performs this step, but there is no way for us to see the final (unique) heading
			// ID it generates. That means the "#" anchor link we generate, and the table of
			// contents, would use the non-unique heading ID. Generating the heading ID ourselves
			// fixes these issues.
			node.HeadingID = r.ensureUniqueHeadingID(node.HeadingID)
		}

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
				fmt.Fprintf(w, `<a name="%s" class="anchor" href="#%s" rel="nofollow" aria-hidden="true"></a>`, node.HeadingID, node.HeadingID)
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
			if v, err := evalMarkdownFuncs(ctx, node.Literal, r.Options); err == nil {
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
		if asideClass := parseAside(node.FirstChild.FirstChild.Literal); asideClass != "" {
			if entering {
				fmt.Fprintf(w, "<aside class=\"%s\">\n", asideClass)
			} else {
				fmt.Fprint(w, "</aside>\n")
			}
			return blackfriday.GoToNext
		}
	}
	return r.Renderer.RenderNode(w, node, entering)
}

func (r *renderer) ensureUniqueHeadingID(id string) string {
	// Copied from blackfriday.
	for count, found := r.headingIDs[id]; found; count, found = r.headingIDs[id] {
		tmp := fmt.Sprintf("%s-%d", id, count+1)

		if _, tmpFound := r.headingIDs[tmp]; !tmpFound {
			r.headingIDs[id] = count + 1
			id = tmp
		} else {
			id = id + "-1"
		}
	}

	if _, found := r.headingIDs[id]; !found {
		r.headingIDs[id] = 0
	}

	return id
}

func getTitle(node *blackfriday.Node) string {
	if node.Type == blackfriday.Document {
		node = node.FirstChild
	}
	if node != nil && node.Type == blackfriday.Heading && node.HeadingData.Level == 1 {
		return renderText(node)
	}
	return ""
}

func renderText(node *blackfriday.Node) string {
	var parts [][]byte
	node.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if node.Type == blackfriday.Text {
			parts = append(parts, node.Literal)
		}
		return blackfriday.GoToNext
	})
	return string(bytes.Join(parts, nil))
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

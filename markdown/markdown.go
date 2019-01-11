package markdown

import (
	"bytes"
	"fmt"
	"io"
	"net/url"

	"github.com/Depado/bfchroma"
	"github.com/alecthomas/chroma/styles"
	"github.com/shurcooL/sanitized_anchor_name"
	blackfriday "gopkg.in/russross/blackfriday.v2"
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

// Run parses and HTML-renders a Markdown document (with optional metadata in the Markdown "front
// matter").
func Run(input []byte, opt Options) (*Document, error) {
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
	ast.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		return renderer.RenderNode(&buf, node, entering)
	})

	doc := Document{
		Meta: meta,
		HTML: buf.Bytes(),
		Tree: newTree(ast),
	}
	if meta.Title != "" {
		doc.Title = meta.Title
	} else {
		doc.Title = getTitle(ast)
	}
	return &doc, nil
}

type renderer struct {
	Options
	blackfriday.Renderer
}

func (r *renderer) RenderNode(w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	switch node.Type {
	case blackfriday.Heading:
		// Make the heading ID based on the text contents, not the raw contents.
		node.HeadingID = sanitized_anchor_name.Create(renderText(node))

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
			if err == nil && !dest.IsAbs() {
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

func getTitle(node *blackfriday.Node) string {
	if node.Type == blackfriday.Document {
		node = node.FirstChild
	}
	if node.Type == blackfriday.Heading && node.HeadingData.Level == 1 {
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

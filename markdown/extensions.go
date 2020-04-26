package markdown

import (
	"bytes"
	"context"
	"fmt"
	"net/url"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var _ renderer.NodeRenderer = (*headingNodeRenderer)(nil)

type headingNodeRenderer struct{}

// Copied from https://github.com/yuin/goldmark/blob/a302193b064875a8af8cd241985cb26574f37408/renderer/html/html.go#L227
func (r *headingNodeRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	if entering {
		_, _ = w.WriteString("<h")
		_ = w.WriteByte("0123456"[n.Level])
		if n.Attributes() != nil {
			html.RenderAttributes(w, node, html.HeadingAttributeFilter)
		}
		_ = w.WriteByte('>')
	} else {
		_, _ = w.WriteString("</h")
		_ = w.WriteByte("0123456"[n.Level])
		_, _ = w.WriteString(">\n")
	}
	return ast.WalkContinue, nil
}

func (r *headingNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		status, err := r.render(w, source, node, entering)
		if err != nil || status != ast.WalkContinue {
			return status, err
		}

		if !entering {
			return ast.WalkContinue, nil
		}

		n := node.(*ast.Heading)

		// Add "#" anchor links to headers to make it easy for users to discover and copy links
		// to sections of a document.
		attrID := GetAttributeID(n)

		// If heading consists only of a link, do not emit an anchor link.
		if hasSingleChildOfLink(n) {
			_, _ = fmt.Fprintf(w, `<a name="%s" aria-hidden="true"></a>`, attrID)
		} else {
			_, _ = fmt.Fprintf(w, `<a name="%[1]s" class="anchor" href="#%[1]s" rel="nofollow" aria-hidden="true" title="#%[1]s"></a>`, attrID)
		}
		return ast.WalkContinue, nil
	})
}

func GetAttributeID(node ast.Node) string {
	attr, ok := node.AttributeString("id")
	if !ok {
		return ""
	}

	v, ok := attr.([]byte)
	if !ok {
		return ""
	}
	return string(v)
}

func hasSingleChildOfLink(node ast.Node) bool {
	seenLink := false
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch {
		case child.Kind() == ast.KindText && child.(*ast.Text).Segment.Len() == 0:
			continue
		case child.Kind() == ast.KindLink && !seenLink:
			seenLink = true
		default:
			return false
		}
	}
	return seenLink
}

var _ renderer.NodeRenderer = (*htmlBlockNodeRenderer)(nil)

type htmlBlockNodeRenderer struct {
	Options
}

func (r *htmlBlockNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		n := node.(*ast.HTMLBlock)

		var val []byte
		for i := 0; i < n.Lines().Len(); i++ {
			s := n.Lines().At(i)
			val = append(val, s.Value(source)...)
		}

		// Rewrite URLs correctly when they are relative to the document, regardless of whether it's
		// an index.md document or not.
		if entering && r.Options.Base != nil {
			if v, err := rewriteRelativeURLsInHTML(val, r.Options); err == nil {
				val = v
			}
		}
		// Evaluate Markdown funcs (<div markdown-func=name ...> nodes), using a heuristic to
		// skip blocks that don't contain any invocations.
		if entering {
			if v, err := EvalMarkdownFuncs(context.Background(), val, r.Options); err == nil {
				val = v
			} else {
				return ast.WalkStop, err
			}

			_, _ = w.Write(val)
		} else {
			if n.HasClosure() {
				closure := n.ClosureLine
				_, _ = w.Write(closure.Value(source))
			}
		}
		return ast.WalkContinue, nil
	})
}

var _ renderer.NodeRenderer = (*blockQuoteNodeRenderer)(nil)

type blockQuoteNodeRenderer struct{}

func (r *blockQuoteNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindBlockquote, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
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

		n := node.(*ast.Blockquote)

		paragraph := n.FirstChild()
		var val []byte
		for i := 0; i < paragraph.Lines().Len(); i++ {
			s := paragraph.Lines().At(i)
			val = append(val, s.Value(source)...)
		}

		if asideClass := parseAside(val); asideClass != "" {
			if entering {
				_, _ = fmt.Fprintf(w, "<aside class=\"%s\">\n", asideClass)
			} else {
				_, _ = fmt.Fprint(w, "</aside>\n")
			}
		}
		return ast.WalkContinue, nil
	})
}

var _ renderer.NodeRenderer = (*linkAndImageNodeRenderer)(nil)

type linkAndImageNodeRenderer struct {
	Options
	Unsafe bool
	XHTML  bool
}

// Copied from https://github.com/yuin/goldmark/blob/a302193b064875a8af8cd241985cb26574f37408/renderer/html/html.go#L516
func (r *linkAndImageNodeRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if entering {
		_, _ = w.WriteString("<a href=\"")
		if r.Unsafe || !html.IsDangerousURL(n.Destination) {
			_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
		}
		_ = w.WriteByte('"')
		if n.Title != nil {
			_, _ = w.WriteString(` title="`)
			_, _ = w.Write(n.Title)
			_ = w.WriteByte('"')
		}
		if n.Attributes() != nil {
			html.RenderAttributes(w, n, html.LinkAttributeFilter)
		}
		_ = w.WriteByte('>')
	} else {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

// Copied from https://github.com/yuin/goldmark/blob/a302193b064875a8af8cd241985cb26574f37408/renderer/html/html.go#L557
func (r *linkAndImageNodeRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	_, _ = w.WriteString("<img src=\"")
	if r.Unsafe || !html.IsDangerousURL(n.Destination) {
		_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
	}
	_, _ = w.WriteString(`" alt="`)
	_, _ = w.Write(n.Text(source))
	_ = w.WriteByte('"')
	if n.Title != nil {
		_, _ = w.WriteString(` title="`)
		_, _ = w.Write(n.Title)
		_ = w.WriteByte('"')
	}
	if n.Attributes() != nil {
		html.RenderAttributes(w, n, html.LinkAttributeFilter)
	}
	if r.XHTML {
		_, _ = w.WriteString(" />")
	} else {
		_, _ = w.WriteString(">")
	}
	return ast.WalkSkipChildren, nil
}

func (r *linkAndImageNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	fn := func(w util.BufWriter, source []byte, node ast.Node, entering bool) (status ast.WalkStatus, err error) {
		var dest string
		switch n := node.(type) {
		case *ast.Link:
			dest = string(n.Destination)
		case *ast.Image:
			dest = string(n.Destination)
		default:
			panic("unreachable")
		}

		if entering {
			destURL, err := url.Parse(dest)
			if err == nil && !destURL.IsAbs() && destURL.Path != "" {
				if r.Options.ContentFilePathToLinkPath != nil {
					destURL.Path = r.Options.ContentFilePathToLinkPath(destURL.Path)
				}
				if r.Options.Base != nil {
					destURL = r.Options.Base.ResolveReference(destURL)
				}
				dest = destURL.String()
			}
		}

		switch n := node.(type) {
		case *ast.Link:
			n.Destination = []byte(dest)
			status, err = r.renderLink(w, source, n, entering)
		case *ast.Image:
			n.Destination = []byte(dest)
			status, err = r.renderImage(w, source, n, entering)
		default:
			panic("unreachable")
		}
		return status, err
	}
	reg.Register(ast.KindLink, fn)
	reg.Register(ast.KindImage, fn)
}

var _ goldmark.Extender = (*extender)(nil)

type extender struct {
	Options
}

func (e *extender) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&headingNodeRenderer{}, 0),
		util.Prioritized(&htmlBlockNodeRenderer{Options: e.Options}, 0),
		util.Prioritized(&blockQuoteNodeRenderer{}, 0),
		util.Prioritized(&linkAndImageNodeRenderer{Options: e.Options, Unsafe: true, XHTML: true}, 0),
	))
}

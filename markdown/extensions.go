package markdown

import (
	"bytes"
	"context"
	"fmt"

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

var _ goldmark.Extender = (*extender)(nil)

type extender struct {
	Options
}

func (e *extender) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&headingNodeRenderer{}, 0),
		util.Prioritized(&htmlBlockNodeRenderer{Options: e.Options}, 0),
		util.Prioritized(&blockQuoteNodeRenderer{}, 0),
	))
}

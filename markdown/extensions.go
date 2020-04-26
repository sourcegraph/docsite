package markdown

import (
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
		attrID := getAttributeID(n)

		// If heading consists only of a link, do not emit an anchor link.
		if hasSingleChildOfLink(n) {
			_, _ = fmt.Fprintf(w, `<a name="%s" aria-hidden="true"></a>`, attrID)
		} else {
			_, _ = fmt.Fprintf(w, `<a name="%[1]s" class="anchor" href="#%[1]s" rel="nofollow" aria-hidden="true" title="#%[1]s"></a>`, attrID)
		}
		return ast.WalkContinue, nil
	})
}

func getAttributeID(node ast.Node) string {
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

var _ goldmark.Extender = (*extender)(nil)

type extender struct {
	Options
}

func (e *extender) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&headingNodeRenderer{}, 0),
	))
}

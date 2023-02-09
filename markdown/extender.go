package markdown

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var _ goldmark.Extender = (*extender)(nil)

type extender struct {
	Options
}

func (e *extender) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&nodeRenderer{e.Options}, 10),
		),
	)
}

var _ renderer.NodeRenderer = (*nodeRenderer)(nil)

type nodeRenderer struct {
	Options
}

func (r *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		n := node.(*ast.Heading)
		if !entering {
			_, _ = w.WriteString("</h")
			_ = w.WriteByte("0123456"[n.Level])
			_, _ = w.WriteString(">\n")
			return ast.WalkContinue, nil
		}

		_, _ = w.WriteString("<h")
		_ = w.WriteByte("0123456"[n.Level])
		if n.Attributes() != nil {
			goldmarkhtml.RenderAttributes(w, node, goldmarkhtml.HeadingAttributeFilter)
		}
		_ = w.WriteByte('>')

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
	reg.Register(ast.KindHTMLBlock, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		n := node.(*ast.HTMLBlock)
		if !entering {
			if n.HasClosure() {
				val := n.ClosureLine.Value(source)
				// For unknown reason, goldmark would write closure for HTML comment twice.
				if !bytes.Contains(val, []byte("-->")) {
					_, _ = w.Write(val)
				}
			}
			return ast.WalkContinue, nil
		}

		var val []byte
		for i := 0; i < n.Lines().Len(); i++ {
			s := n.Lines().At(i)
			val = append(val, s.Value(source)...)
		}

		if entering {
			// Rewrite URLs correctly when they are relative to the document, regardless of whether it's
			// an index.md document or not.
			if r.Options.Base != nil {
				if v, err := rewriteRelativeURLsInHTML(val, r.Options); err == nil {
					val = v
				}
			}

			// Evaluate Markdown funcs (<div markdown-func=name ...> nodes), using a heuristic to
			// skip blocks that don't contain any invocations.
			if v, err := EvalMarkdownFuncs(context.Background(), val, r.Options); err == nil {
				val = v
			} else {
				return ast.WalkStop, err
			}

			_, _ = w.Write(val)
		} else if n.HasClosure() {
			_, _ = w.Write(n.ClosureLine.Value(source))
		}
		return ast.WalkContinue, nil
	})
	reg.Register(ast.KindRawHTML, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkSkipChildren, nil
		}

		n := node.(*ast.RawHTML)

		var val []byte
		l := n.Segments.Len()
		for i := 0; i < l; i++ {
			segment := n.Segments.At(i)
			val = append(val, segment.Value(source)...)
		}

		// Rewrite URLs correctly when they are relative to the document, regardless of whether it's
		// an index.md document or not.
		if r.Options.Base != nil {
			if v, err := rewriteRelativeURLsInHTML(val, r.Options); err == nil {
				val = v
			}
		}
		_, _ = w.Write(val)
		return ast.WalkSkipChildren, nil
	})
	reg.Register(ast.KindBlockquote, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		n := node.(*ast.Blockquote)
		paragraph := n.FirstChild()
		var val []byte
		for i := 0; i < paragraph.Lines().Len(); i++ {
			s := paragraph.Lines().At(i)
			val = append(val, s.Value(source)...)
		}

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
		aside := parseAside(val)
		if aside != "" {
			if entering {
				_, _ = w.WriteString(fmt.Sprintf("<aside class=\"%s\">\n", aside))
			} else {
				_, _ = w.WriteString("</aside>\n")
			}
		} else {
			if entering {
				_, _ = w.WriteString("<blockquote>\n")
			} else {
				_, _ = w.WriteString("</blockquote>\n")
			}
		}
		return ast.WalkContinue, nil
	})

	var anchorDirectivePattern = regexp.MustCompile(`\{#[\w.-]+\}`)
	reg.Register(ast.KindText, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		n := node.(*ast.Text)
		text := n.Text(source)

		// Rewrites `{#foo}` directives in text to `<a id="foo"></a>` anchors.
		matches := anchorDirectivePattern.FindAllIndex(text, -1)
		if len(matches) > 0 {
			i := 0
			for _, match := range matches {
				start, end := match[0], match[1]
				if i != start {
					_, _ = w.Write(text[i:start])
				}

				escapedID := html.EscapeString(string(text[start+2 : end-1]))
				_, _ = w.WriteString(fmt.Sprintf(`<span id="%[1]s" class="anchor-inline"></span><a href="#%[1]s" class="anchor-inline-link" title="#%[1]s"></a>`, escapedID))
				i = end
			}
			if i != len(text) {
				_, _ = w.Write(text[i:])
			}
			return ast.WalkContinue, nil
		}

		// Marks up strings that look like dates as `<time>` tags, with a
		// machine-readable `datetime` attribute. The client can use this to highlight
		// them with CSS and show the date in the user's date format and timezone with
		// JS.
		matches = datePattern.FindAllIndex(text, -1)
		if len(matches) == 0 {
			_, _ = w.Write(text)
			return ast.WalkContinue, nil
		}

		i := 0
		for _, match := range matches {
			start, end := match[0], match[1]
			if i != start {
				_, _ = w.Write(text[i:start])
			}

			dateStr := string(text[start:end])
			fiscalIntervalStart := parseFiscalInterval(dateStr)
			if fiscalIntervalStart == "" {
				_, _ = w.WriteString(dateStr)
			} else {
				// TODO expose end of interval as data attribute
				_, _ = w.WriteString(fmt.Sprintf(`<time datetime="%s" data-is-start-of-interval="true">%s</time>`, fiscalIntervalStart, dateStr))
			}
			i = end
		}
		if i != len(text) {
			_, _ = w.Write(text[i:])
		}

		return ast.WalkContinue, nil
	})

	renderLinkAndImage := func(w util.BufWriter, source []byte, node ast.Node, entering bool) (status ast.WalkStatus, err error) {
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
	reg.Register(ast.KindLink, renderLinkAndImage)
	reg.Register(ast.KindImage, renderLinkAndImage)
}

// Copied from https://github.com/yuin/goldmark/blob/a302193b064875a8af8cd241985cb26574f37408/renderer/html/html.go#L516
func (r *nodeRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if entering {
		text := n.Text(source)

		// Handle the leading whitespace
		if i := bytes.IndexByte(text, '<'); i > 0 {
			_, _ = w.Write(text[:i])
		}
		_, _ = w.WriteString(`<a href="`)
		if !goldmarkhtml.IsDangerousURL(n.Destination) {
			_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
		}
		_ = w.WriteByte('"')
		if n.Title != nil {
			_, _ = w.WriteString(` title="`)
			_, _ = w.Write(n.Title)
			_ = w.WriteByte('"')
		}
		if n.Attributes() != nil {
			goldmarkhtml.RenderAttributes(w, n, goldmarkhtml.LinkAttributeFilter)
		}
		_ = w.WriteByte('>')
	} else {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

// Copied from https://github.com/yuin/goldmark/blob/a302193b064875a8af8cd241985cb26574f37408/renderer/html/html.go#L557
func (r *nodeRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	_, _ = w.WriteString("<img src=\"")
	if !goldmarkhtml.IsDangerousURL(n.Destination) {
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
		goldmarkhtml.RenderAttributes(w, n, goldmarkhtml.LinkAttributeFilter)
	}

	_, _ = w.WriteString(">")
	return ast.WalkSkipChildren, nil
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

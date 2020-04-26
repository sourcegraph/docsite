package markdown

import (
	"strings"

	"github.com/yuin/goldmark/ast"
)

// SectionNode is a section and its children.
type SectionNode struct {
	Title    string         // section title
	URL      string         // section URL (usually an anchor link)
	Level    int            // heading level (1–6)
	Children []*SectionNode // subsections
}

func newTree(node ast.Node, source []byte) []*SectionNode {
	stack := []*SectionNode{{}}
	cur := func() *SectionNode { return stack[len(stack)-1] }
	ast.Walk(node, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Kind() != ast.KindHeading {
			return ast.WalkContinue, nil
		}

		n := node.(*ast.Heading)

		for n.Level <= cur().Level {
			stack = stack[:len(stack)-1]
		}

		// If heading consists only of a link, use the link URL (not the heading ID) as the
		// destination.
		var url string
		if hasSingleChildOfLink(n) {
			if link := getFirstChildLink(node); link != nil && len(link.Destination) > 0 {
				url = string(link.Destination)
			}
		}
		if url == "" {
			url = "#" + getAttributeID(n)
		}

		sn := &SectionNode{
			Title: strings.ReplaceAll(string(RenderText(n, source)), "`", ""),
			URL:   url,
			Level: n.Level,
		}
		cur().Children = append(cur().Children, sn)
		stack = append(stack, sn)
		return ast.WalkContinue, nil
	})
	return stack[0].Children
}

func getFirstChildLink(node ast.Node) *ast.Link {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind() == ast.KindLink {
			return child.(*ast.Link)
		}
	}
	return nil
}

package markdown

import blackfriday "gopkg.in/russross/blackfriday.v2"

// SectionNode is a section and its children.
type SectionNode struct {
	Title    string         // section title
	URL      string         // section URL (usually an anchor link)
	Level    int            // heading level (1–6)
	Children []*SectionNode // subsections
}

func newTree(node *blackfriday.Node) []*SectionNode {
	stack := []*SectionNode{{}}
	cur := func() *SectionNode { return stack[len(stack)-1] }
	node.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if entering && node.Type == blackfriday.Heading {
			for node.Level <= cur().Level {
				stack = stack[:len(stack)-1]
			}

			// If heading consists only of a link, use the link URL (not the heading ID) as the
			// destination.
			var url string
			if hasSingleChildOfType(node, blackfriday.Link) {
				if link := getFirstChildLink(node); link != nil && len(link.LinkData.Destination) > 0 {
					url = string(link.LinkData.Destination)
				}
			}
			if url == "" {
				url = "#" + node.HeadingID
			}

			n := &SectionNode{Title: renderText(node), URL: url, Level: node.Level}
			cur().Children = append(cur().Children, n)
			stack = append(stack, n)
		}
		return blackfriday.GoToNext
	})
	return stack[0].Children
}

package markdown

import (
	"fmt"

	"github.com/russross/blackfriday/v2"
	"github.com/shurcooL/sanitized_anchor_name"
)

// SetHeadingIDs sets the HeadingID for each heading node.
func SetHeadingIDs(node *blackfriday.Node) {
	headingIDs := map[string]int{} // for generating unique heading IDs
	ensureUniqueHeadingID := func(id string) string {
		// Copied from blackfriday.
		for count, found := headingIDs[id]; found; count, found = headingIDs[id] {
			tmp := fmt.Sprintf("%s-%d", id, count+1)

			if _, tmpFound := headingIDs[tmp]; !tmpFound {
				headingIDs[id] = count + 1
				id = tmp
			} else {
				id = id + "-1"
			}
		}

		if _, found := headingIDs[id]; !found {
			headingIDs[id] = 0
		}

		return id
	}

	node.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if entering && node.Type == blackfriday.Heading {
			// Make the heading ID based on the text contents, not the raw contents. (But keep the
			// heading ID explicitly specified with `# foo {:#myid}`, if any.)
			if hasExplicitHeadingID := node.HeadingID != ""; !hasExplicitHeadingID {
				node.HeadingID = sanitized_anchor_name.Create(string(RenderTextOld(node)))
			}

			// Ensure the heading ID is unique. The blackfriday package (in ensureUniqueHeadingID)
			// also performs this step, but there is no way for us to see the final (unique) heading
			// ID it generates. That means the "#" anchor link we generate, and the table of
			// contents, would use the non-unique heading ID. Generating the heading ID ourselves
			// fixes these issues.
			node.HeadingID = ensureUniqueHeadingID(node.HeadingID)
		}
		return blackfriday.GoToNext
	})
}

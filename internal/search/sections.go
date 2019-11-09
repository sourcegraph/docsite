package search

import (
	"github.com/sourcegraph/docsite/internal/search/query"
	"github.com/sourcegraph/docsite/markdown"
	"gopkg.in/russross/blackfriday.v2"
)

type SectionResult struct {
	ID       string   // the URL fragment (without "#") of the section, or empty if in the first section
	Stack    []string // the stack of section IDs
	Excerpts []string // the match excerpt
}

func documentSectionResults(data []byte, query query.Query) ([]SectionResult, error) {
	type stackEntry struct {
		id    string
		level int
	}
	stack := []stackEntry{{}}
	cur := func() stackEntry { return stack[len(stack)-1] }
	ast := markdown.NewParser(markdown.NewBfRenderer()).Parse(data)
	markdown.SetHeadingIDs(ast)

	var results []SectionResult
	addResult := func(excerpts []string) {
		stackIDs := make([]string, len(stack))
		for i, e := range stack {
			stackIDs[i] = e.id
		}
		results = append(results, SectionResult{
			ID:       cur().id,
			Stack:    stackIDs,
			Excerpts: excerpts,
		})
	}

	ast.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if entering && node.Type == blackfriday.Heading {
			for node.Level <= cur().level {
				stack = stack[:len(stack)-1]
			}

			// For the document title heading, use the empty ID.
			var id string
			if !markdown.IsDocumentTitleHeadingNode(node) {
				id = node.HeadingID
			}

			stack = append(stack, stackEntry{id: id, level: node.Level})
		}

		if entering && (node.Type == blackfriday.Paragraph || node.Type == blackfriday.Item || node.Type == blackfriday.Heading || node.Type == blackfriday.BlockQuote || node.Type == blackfriday.Code) {
			text := markdown.RenderText(node)
			if matches := query.FindAllIndex(text); len(matches) > 0 {
				// Don't include excerpts for heading because all of the heading is considered the
				// match.
				var excerpts []string
				if node.Type != blackfriday.Heading {
					excerpts = make([]string, len(matches))
					for i, match := range matches {
						const excerptMaxLength = 220
						excerpts[i] = excerpt(string(text), match[0], match[1], excerptMaxLength)
					}
				}

				// Remove the previous heading-only match for the same section, if any. A match with
				// an excerpt is strictly better than one without.
				if len(results) > 0 {
					last := results[len(results)-1]
					if last.ID == cur().id && len(last.Excerpts) == 0 {
						results = results[:len(results)-1]
					}
				}

				addResult(excerpts)

				return blackfriday.SkipChildren
			}
		}

		return blackfriday.GoToNext
	})
	return results, nil
}

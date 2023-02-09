package search

import (
	gohtml "html"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/sourcegraph/docsite/internal/search/query"
	"github.com/sourcegraph/docsite/markdown"
)

type SectionResult struct {
	ID         string   // the URL fragment (without "#") of the section, or empty if in the first section
	IDStack    []string // the stack of section IDs
	Title      string   // the section title
	TitleStack []string // the stack of section titles
	Excerpts   []string // the match excerpt
}

func documentSectionResults(source []byte, query query.Query) ([]SectionResult, error) {
	type stackEntry struct {
		id    string
		title string
		level int
	}
	stack := []stackEntry{{}}
	cur := func() stackEntry { return stack[len(stack)-1] }
	root := markdown.New(markdown.Options{}).Parser().Parse(text.NewReader(source))

	var results []SectionResult
	addResult := func(excerpts []string) {
		stackIDs := make([]string, len(stack))
		stackTitles := make([]string, len(stack))
		for i, e := range stack {
			stackIDs[i] = e.id
			stackTitles[i] = e.title
		}
		e := cur()

		// If last section result was in the same section, just append the excerpt instead of
		// creating a new section result.
		if len(results) > 0 {
			last := results[len(results)-1]
			if lastResultIsSameSection := last.ID == e.id; lastResultIsSameSection {
				last.Excerpts = append(last.Excerpts, excerpts...)
				return
			}
		}
		results = append(results, SectionResult{
			ID:         e.id,
			IDStack:    stackIDs,
			Title:      e.title,
			TitleStack: stackTitles,
			Excerpts:   excerpts,
		})
	}

	err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if node.Kind() == ast.KindHeading {
			n := node.(*ast.Heading)
			for n.Level <= cur().level {
				stack = stack[:len(stack)-1]
			}

			// For the document top title heading, use the empty ID.
			var id string
			if !markdown.IsDocumentTopTitleHeadingNode(node) {
				id = markdown.GetAttributeID(n)
			}

			stack = append(stack, stackEntry{
				id:    id,
				title: string(n.Text(source)),
				level: n.Level,
			})
		}

		if entering &&
			(node.Kind() == ast.KindParagraph ||
				node.Kind() == ast.KindListItem ||
				node.Kind() == ast.KindHeading ||
				node.Kind() == ast.KindBlockquote ||
				node.Kind() == ast.KindCodeBlock ||
				node.Kind() == ast.KindFencedCodeBlock) {
			text := node.Text(source)
			if matches := query.FindAllIndex(string(text)); len(matches) > 0 {
				// Don't include excerpts for heading because all of the heading is considered the
				// match.
				var excerpts []string
				if node.Kind() != ast.KindHeading {
					excerpts = make([]string, len(matches))
					for i, match := range matches {
						const excerptMaxLength = 220
						excerpts[i] = gohtml.UnescapeString(string(excerpt(text, match[0], match[1], excerptMaxLength)))
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

				return ast.WalkSkipChildren, nil
			}
		}

		return ast.WalkContinue, nil
	})
	return results, err
}

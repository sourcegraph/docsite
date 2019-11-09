package docsite

import (
	"bytes"
	"context"
	"html"
	"html/template"
	"strings"

	"github.com/sourcegraph/docsite/internal/search"
	"github.com/sourcegraph/docsite/internal/search/index"
	"github.com/sourcegraph/docsite/internal/search/query"
)

// Search searches all documents at the version for a query.
func (s *Site) Search(ctx context.Context, contentVersion string, queryStr string) (*search.Result, error) {
	pages, err := s.AllContentPages(ctx, contentVersion)
	if err != nil {
		return nil, err
	}

	idx, err := index.New()
	if err != nil {
		return nil, err
	}
	for _, page := range pages {
		if err := idx.Add(ctx, index.Document{
			ID:   index.DocID(page.FilePath),
			Data: page.Data,
		}); err != nil {
			return nil, err
		}
	}

	return search.Search(ctx, query.Parse(queryStr), idx)
}

func (s *Site) renderSearchPage(queryStr string, result *search.Result) ([]byte, error) {
	query := query.Parse(queryStr)
	tmpl, err := s.getTemplate(s.Templates, searchTemplateName, template.FuncMap{
		"nomd": func(s string) string { return strings.TrimSuffix(strings.TrimSuffix(s, ".md"), "/index") },
		"highlight": func(text string) template.HTML {
			var s []string
			c := 0
			for _, match := range query.FindAllIndex([]byte(text)) {
				start, end := match[0], match[1]
				if start > c {
					s = append(s, html.EscapeString(text[c:start]))
				}
				s = append(s, "<strong>"+html.EscapeString(text[start:end])+"</strong>")
				c = end
			}
			if c < len(text) {
				s = append(s, html.EscapeString(text[c:]))
			}
			return template.HTML(strings.Join(s, ""))
		},
	})
	if err != nil {
		return nil, err
	}

	data := struct {
		Query  string
		Result *search.Result
	}{
		Query:  queryStr,
		Result: result,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

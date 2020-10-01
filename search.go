package docsite

import (
	"bytes"
	"context"
	"html"
	"html/template"
	"net/url"
	"sort"
	"strings"

	"github.com/russross/blackfriday/v2"
	"github.com/sourcegraph/docsite/internal/search"
	"github.com/sourcegraph/docsite/internal/search/index"
	"github.com/sourcegraph/docsite/internal/search/query"
	"github.com/sourcegraph/docsite/markdown"
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
		ast := markdown.NewParser(nil).Parse(page.Data)
		data, err := s.renderTextContent(ctx, page, ast, contentVersion)
		if err != nil {
			return nil, err
		}

		if err := idx.Add(ctx, index.Document{
			ID:    index.DocID(page.FilePath),
			Title: markdown.GetTitle(ast),
			URL:   s.Base.ResolveReference(&url.URL{Path: page.Path}).String(),
			Data:  string(data),
		}); err != nil {
			return nil, err
		}
	}

	return search.Search(ctx, query.Parse(queryStr), idx)
}

func (s *Site) renderTextContent(ctx context.Context, page *ContentPage, ast *blackfriday.Node, contentVersion string) ([]byte, error) {
	// Evaluate <div markdown-func> elements if present (use a heuristic to determine if
	// present, to avoid needless work).
	maybeHasMarkdownFunc := bytes.Contains(page.Data, []byte("markdown-func"))
	if !maybeHasMarkdownFunc {
		return page.Data, nil
	}

	opt := s.markdownOptions(page.FilePath, contentVersion)
	ast.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		switch node.Type {
		case blackfriday.HTMLBlock, blackfriday.HTMLSpan:
			if entering {
				if v, err := markdown.EvalMarkdownFuncs(ctx, node.Literal, opt); err == nil {
					page.Data = bytes.Replace(page.Data, node.Literal, v, 1)
				}
			}
		}
		return blackfriday.GoToNext
	})

	return page.Data, nil
}

func (s *Site) renderSearchPage(contentVersion, queryStr string, result *search.Result) ([]byte, error) {
	query := query.Parse(queryStr)
	tmpl, err := s.getTemplate(s.Templates, searchTemplateName, template.FuncMap{
		"highlight": func(text string) template.HTML { return highlight(query, text) },
	})
	if err != nil {
		return nil, err
	}

	data := struct {
		ContentVersion string
		Query          string
		Result         *search.Result
	}{
		ContentVersion: contentVersion,
		Query:          queryStr,
		Result:         result,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// highlight returns an HTML fragment with matches of the pattern in text wrapped in <strong>.
func highlight(query query.Query, text string) template.HTML {
	// Highlight longer matches first (among matches starting at the same position).
	matches := query.FindAllIndex(text)
	sort.Slice(matches, func(i, j int) bool {
		return (matches[i][0] < matches[j][0]) || (matches[i][0] == matches[j][0] && matches[i][1] > matches[j][1])
	})

	var s []string
	c := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		if start > c {
			s = append(s, html.EscapeString(text[c:start]))
		}
		if start < c {
			start = c
		}
		if start < end {
			s = append(s, "<strong>"+html.EscapeString(text[start:end])+"</strong>")
		}
		if end > c {
			c = end
		}
	}
	if c < len(text) {
		s = append(s, html.EscapeString(text[c:]))
	}
	return template.HTML(strings.Join(s, ""))
}

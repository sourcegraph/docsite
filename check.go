package docsite

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"

	"github.com/russross/blackfriday/v2"
	"github.com/sourcegraph/docsite/markdown"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Check checks the site content for common problems (such as broken links).
func (s *Site) Check(ctx context.Context, contentVersion string) (problems []string, err error) {
	pages, err := s.AllContentPages(ctx, contentVersion)
	if err != nil {
		return nil, err
	}

	problemPrefix := func(page *ContentPage) string {
		return fmt.Sprintf("%s: ", page.FilePath)
	}

	// Render and parse the pages.
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	addProblem := func(problem string) {
		mu.Lock()
		problems = append(problems, problem)
		mu.Unlock()
	}
	allPageDatas := make([]*contentPageCheckData, 0, len(pages))
	for _, page := range pages {
		wg.Add(1)
		go func(page *ContentPage) {
			defer wg.Done()
			data, err := s.RenderContentPage(&PageData{Content: page})
			if err != nil {
				addProblem(problemPrefix(page) + err.Error())
				return
			}
			doc, err := html.Parse(bytes.NewReader(data))
			if err != nil {
				addProblem(problemPrefix(page) + err.Error())
				return
			}
			pageData := &contentPageCheckData{
				ContentPage: page,
				doc:         doc,
			}

			mu.Lock()
			allPageDatas = append(allPageDatas, pageData)
			mu.Unlock()

			// Find per-page problems.
			pageProblems := s.checkContentPage(pageData)
			for _, p := range pageProblems {
				addProblem(problemPrefix(pageData.ContentPage) + p)
			}
		}(page)
	}
	wg.Wait()

	// Find site-wide problems.
	problems = append(problems, s.checkSite(allPageDatas)...)

	return problems, nil
}

type contentPageCheckData struct {
	*ContentPage
	doc *html.Node
}

func (s *Site) checkContentPage(page *contentPageCheckData) (problems []string) {
	// Find invalid links.
	ast := markdown.NewParser(markdown.NewBfRenderer()).Parse(page.Data)
	ast.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if entering && (node.Type == blackfriday.Link || node.Type == blackfriday.Image) {
			u, err := url.Parse(string(node.LinkData.Destination))
			if err != nil {
				problems = append(problems, fmt.Sprintf("invalid URL %q", node.LinkData.Destination))
				return blackfriday.GoToNext
			}

			isPathOnly := u.Scheme == "" && u.Host == ""

			// Reject absolute paths because they will break when browsing the docs on
			// GitHub/Sourcegraph in the repository, or if the root path ever changes.
			if isPathOnly && strings.HasPrefix(u.Path, "/") {
				problems = append(problems, fmt.Sprintf("must use relative, not absolute, link to %s", node.LinkData.Destination))
			}

			if node.Type == blackfriday.Link {
				// Require that relative paths link to the actual .md file, so that browsing
				// docs on the file system works.
				if isPathOnly && u.Path != "" && !strings.HasSuffix(u.Path, ".md") {
					problems = append(problems, fmt.Sprintf("must link to .md file, not %s", u.Path))
				}
			}
		}

		return blackfriday.GoToNext
	})

	// Find broken links.
	handler := s.Handler()
	walkOpt := walkHTMLDocumentOptions{
		url: func(urlStr string) {
			if s.CheckIgnoreURLPattern != nil && s.CheckIgnoreURLPattern.MatchString(urlStr) {
				return
			}

			if _, err := url.Parse(urlStr); err != nil {
				problems = append(problems, fmt.Sprintf("invalid URL %q", urlStr))
			}

			rr := httptest.NewRecorder()
			req, err := http.NewRequest("HEAD", urlStr, nil)
			if err != nil {
				problems = append(problems, fmt.Sprintf("invalid request URI %q", urlStr))
				return
			}
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				problems = append(problems, fmt.Sprintf("broken link to %s", urlStr))
			}
		},
	}
	walkHTMLDocument(page.doc, walkOpt)

	return problems
}

func (s *Site) checkSite(pages []*contentPageCheckData) (problems []string) {
	inlinks := map[string]struct{}{}
	for _, page := range pages {
		walkHTMLDocument(page.doc, walkHTMLDocumentOptions{
			url: func(urlStr string) {
				u, err := url.Parse(urlStr)
				if err != nil {
					return // invalid URL error will be reported in per-page check
				}
				pagePath := strings.TrimPrefix(u.Path, s.Base.Path)
				if pagePath == page.Path {
					return // ignore self links for the sake of disconnected page detection
				}
				inlinks[pagePath] = struct{}{}
			},
		})
	}

	for _, page := range pages {
		if _, hasInlinks := inlinks[page.Path]; !hasInlinks && page.FilePath != "index.md" && !page.Doc.Meta.IgnoreDisconnectedPageCheck {
			problems = append(problems, fmt.Sprintf("%s: disconnected page (no inlinks from other pages)", page.FilePath))
		}
	}

	return problems
}

type walkHTMLDocumentOptions struct {
	url func(url string) // called for each URL encountered
}

func walkHTMLDocument(node *html.Node, opt walkHTMLDocumentOptions) {
	if node.Type == html.ElementNode {
		switch node.DataAtom {
		case atom.A:
			if href, ok := getAttribute(node, "href"); ok {
				opt.url(href)
			}
		case atom.Img:
			if src, ok := getAttribute(node, "src"); ok {
				opt.url(src)
			}
		}
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		walkHTMLDocument(c, opt)
	}
}

func getAttribute(n *html.Node, key string) (string, bool) {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

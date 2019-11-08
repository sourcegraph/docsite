package docsite

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/sourcegraph/docsite/markdown"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	blackfriday "gopkg.in/russross/blackfriday.v2"
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
	pageData := make([]*contentPageCheckData, 0, len(pages))
	for _, page := range pages {
		data, err := s.RenderContentPage(&PageData{Content: page})
		if err != nil {
			problems = append(problems, problemPrefix(page)+err.Error())
			continue
		}
		doc, err := html.Parse(bytes.NewReader(data))
		if err != nil {
			problems = append(problems, problemPrefix(page)+err.Error())
			continue
		}
		pageData = append(pageData, &contentPageCheckData{
			ContentPage: page,
			doc:         doc,
		})
	}

	// Find per-page problems.
	for _, page := range pageData {
		pageProblems := s.checkContentPage(page)
		for _, p := range pageProblems {
			problems = append(problems, problemPrefix(page.ContentPage)+p)
		}
	}

	// Find site-wide problems.
	problems = append(problems, s.checkSite(pageData)...)

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
		if entering {
			if node.Type == blackfriday.Link || node.Type == blackfriday.Image {
				// Reject absolute paths because they will break when browsing the docs on
				// GitHub/Sourcegraph in the repository, or if the root path ever changes.
				if bytes.HasPrefix(node.LinkData.Destination, []byte("/")) {
					problems = append(problems, fmt.Sprintf("must use relative, not absolute, link to %s", node.LinkData.Destination))
				}
			}

			if node.Type == blackfriday.Link {
				// Require that relative paths link to the actual .md file, so that browsing
				// docs on the file system works.
				u, err := url.Parse(string(node.LinkData.Destination))
				if err != nil {
					problems = append(problems, fmt.Sprintf("invalid URL %q", node.LinkData.Destination))
				} else if !u.IsAbs() && u.Path != "" && !strings.HasSuffix(u.Path, ".md") {
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
			req, _ := http.NewRequest("HEAD", urlStr, nil)
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

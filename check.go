package docsite

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/russross/blackfriday"
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
	for _, page := range pages {
		problemPrefix := fmt.Sprintf("%s: ", page.FilePath)
		pageProblems, pageErr := s.checkContentPage(page)
		if pageErr != nil {
			problems = append(problems, problemPrefix+pageErr.Error())
		}
		for _, p := range pageProblems {
			problems = append(problems, problemPrefix+p)
		}
	}
	return problems, nil
}

func (s *Site) checkContentPage(page *ContentPage) (problems []string, err error) {
	ast := markdown.NewParser().Parse(page.Data)
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

	data, err := s.RenderContentPage(page)
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
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
	walkHTMLDocument(doc, walkOpt)

	return problems, nil
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

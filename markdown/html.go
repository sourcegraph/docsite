package markdown

import (
	"bytes"
	"net/url"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func rewriteRelativeURLsInHTML(htmlSource []byte, opt Options) ([]byte, error) {
	// Pass dummyElement to html.ParseFragment to avoid introducing <html>, <head>, and <body>
	// elements in the final html.Render call.
	dummyElement := &html.Node{Type: html.ElementNode}
	nodes, err := html.ParseFragment(bytes.NewReader(htmlSource), dummyElement)
	if err != nil {
		return nil, err
	}

	resolveURL := func(urlStr string) string {
		if opt.Base == nil {
			return urlStr
		}
		u, err := url.Parse(urlStr)
		if err != nil || u.IsAbs() {
			return urlStr
		}
		return opt.Base.ResolveReference(u).String()
	}

	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.DataAtom {
			case atom.A:
				for i, attr := range node.Attr {
					if attr.Key == "href" {
						attr.Val = resolveURL(attr.Val)
						node.Attr[i] = attr
					}
				}
			case atom.Img:
				for i, attr := range node.Attr {
					if attr.Key == "src" {
						attr.Val = resolveURL(attr.Val)
						node.Attr[i] = attr
					}
				}
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, node := range nodes {
		walk(node)
	}

	var buf bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&buf, node); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

package markdown

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// rewriteRelativeURLsInHTML rewrites <a href> and <img src> attribute values in an HTML fragment
// and returns the rewritten HTML fragment. The HTML fragment may contain unclosed tags (which is
// why it uses a tokenizer instead of a parser, which would auto-close tags upon rendering the
// modified tree).
func rewriteRelativeURLsInHTML(htmlFragment []byte, opt Options) ([]byte, error) {
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

	z := html.NewTokenizer(bytes.NewReader(htmlFragment))
	var buf bytes.Buffer
	for {
		tt := z.Next()
		if tt == html.ErrorToken && z.Err() == io.EOF {
			break
		}
		tok := z.Token()
		if tok.Type == html.StartTagToken || tok.Type == html.SelfClosingTagToken {
			switch tok.DataAtom {
			case atom.A:
				for i, attr := range tok.Attr {
					if attr.Key == "href" && !strings.HasPrefix(attr.Val, "#") {
						attr.Val = resolveURL(attr.Val)
						tok.Attr[i] = attr
					}
				}
			case atom.Img:
				for i, attr := range tok.Attr {
					if attr.Key == "src" {
						attr.Val = resolveURL(attr.Val)
						tok.Attr[i] = attr
					}
				}
			}
		}

		buf.WriteString(tokenStringWithUnescapedText(tok))
	}
	return buf.Bytes(), nil
}

// tokenStringWithUnescapedText returns the HTML token string, except that if it is a text node, it
// is not escaped. For example, a text node containing "&" will not have that character escaped to
// "&amp;". This prevents double-escaping of tokens when multiple HTML tokenizers run on the
// content.
func tokenStringWithUnescapedText(tok html.Token) string {
	if tok.Type == html.TextToken {
		return tok.Data
	}
	return tok.String()
}

// isOnlyHTMLComment reports whether htmlFragment consists only of zero or more HTML comments and whitespace.
func isOnlyHTMLComment(htmlFragment []byte) bool {
	z := html.NewTokenizer(bytes.NewReader(htmlFragment))
	for {
		tt := z.Next()
		if tt == html.CommentToken {
			continue
		}
		if tt == html.TextToken {
			tok := z.Token()
			if strings.TrimSpace(tok.Data) == "" {
				continue
			}
		}
		if tt == html.ErrorToken && z.Err() == io.EOF {
			break
		}
		return false
	}
	return true
}

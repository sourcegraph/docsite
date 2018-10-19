package markdown

import (
	"net/url"
	"testing"
)

func TestRewriteRelativeURLsInHTML(t *testing.T) {
	opt := Options{
		Base: &url.URL{Path: "/a/"},
	}

	tests := map[string]string{
		`<a href="./b">b</a><a href="./c/d">c/d</a>`: `<a href="/a/b">b</a><a href="/a/c/d">c/d</a>`,
		`<img src="b/c">`:  `<img src="/a/b/c">`,
		`<img src="b/c"/>`: `<img src="/a/b/c"/>`,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := rewriteRelativeURLsInHTML([]byte(input), opt)
			if err != nil {
				t.Error(err)
			}
			if string(got) != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

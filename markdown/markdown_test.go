package markdown

import (
	"net/url"
	"strings"
	"testing"
)

func TestMarkdown(t *testing.T) {
	got := strings.TrimSpace(string(Run([]byte("Hello world github/linguist#1 **cool**, and #1!"), Options{})))
	want := "<p>Hello world github/linguist#1 <strong>cool</strong>, and #1!</p>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRelativeURL(t *testing.T) {
	got := strings.TrimSpace(string(Run([]byte("[a](./b/c)"), Options{Base: &url.URL{Path: "/d/"}})))
	want := `<p><a href="/d/b/c">a</a></p>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHeadingAnchorLink(t *testing.T) {
	got := strings.TrimSpace(string(Run([]byte(`## A ' B " C & D ? E`), Options{})))
	want := `<h2 id="a-b-c-d-e"><a name="a-b-c-d-e" class="anchor" href="#a-b-c-d-e" rel="nofollow" aria-hidden="true"></a>A &lsquo; B &ldquo; C &amp; D ? E</h2>`
	if got != want {
		t.Errorf("\ngot:  %s\nwant: %s", got, want)
	}
}

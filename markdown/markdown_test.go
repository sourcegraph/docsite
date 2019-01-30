package markdown

import (
	"bytes"
	"net/url"
	"reflect"
	"testing"
)

func check(t *testing.T, got, want Document) {
	t.Helper()
	if !reflect.DeepEqual(got.Meta, want.Meta) {
		t.Errorf("got meta %+v, want %+v", got.Meta, want.Meta)
	}
	if got.Title != want.Title {
		t.Errorf("got title %q, want %q", got.Title, want.Title)
	}
	if !bytes.Equal(got.HTML, want.HTML) {
		t.Errorf("HTML did not match\ngot:  %s\nwant: %s", got.HTML, want.HTML)
	}
}

func TestRun(t *testing.T) {
	t.Run("no metadata", func(t *testing.T) {
		doc, err := Run([]byte(`# My title
Hello world github/linguist#1 **cool**, and #1!`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		check(t, *doc, Document{
			Title: "My title",
			HTML: []byte(`<h1 id="my-title"><a name="my-title" class="anchor" href="#my-title" rel="nofollow" aria-hidden="true"></a>My title</h1>

<p>Hello world github/linguist#1 <strong>cool</strong>, and #1!</p>
`),
		})
	})
	t.Run("metadata", func(t *testing.T) {
		doc, err := Run([]byte(`---
title: Metadata title
---

# Markdown title`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		check(t, *doc, Document{
			Meta:  Metadata{Title: "Metadata title"},
			Title: "Metadata title",
			HTML: []byte(`<h1 id="markdown-title"><a name="markdown-title" class="anchor" href="#markdown-title" rel="nofollow" aria-hidden="true"></a>Markdown title</h1>
`),
		})
	})
}

func TestRenderer(t *testing.T) {
	t.Run("heading anchor link ignores special chars", func(t *testing.T) {
		doc, err := Run([]byte(`## A ' B " C & D ? E`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a-b-c-d-e"><a name="a-b-c-d-e" class="anchor" href="#a-b-c-d-e" rel="nofollow" aria-hidden="true"></a>A &lsquo; B &ldquo; C &amp; D ? E</h2>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("heading anchor link ignores markup", func(t *testing.T) {
		doc, err := Run([]byte(`## [A](B) C`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a-c"><a name="a-c" class="anchor" href="#a-c" rel="nofollow" aria-hidden="true"></a><a href="B">A</a> C</h2>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("disambiguates heading anchor", func(t *testing.T) {
		doc, err := Run([]byte("# A\n\n# A"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h1 id="a"><a name="a" class="anchor" href="#a" rel="nofollow" aria-hidden="true"></a>A</h1>` + "\n\n" + `<h1 id="a-2"><a name="a-2" class="anchor" href="#a-2" rel="nofollow" aria-hidden="true"></a>A</h1>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("syntax highlighting go", func(t *testing.T) {
		doc, err := Run([]byte("```go\nvar foo struct{}\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre style="background-color:#fff"><span style="color:#00f">var</span> foo <span style="color:#00f">struct</span>{}` + "\n" + `</pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("syntax highlighting typescript", func(t *testing.T) {
		doc, err := Run([]byte("```typescript\nconst foo = 'bar'\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre style="background-color:#fff"><span style="color:#00f">const</span> foo = <span style="color:#a31515">&#39;bar&#39;</span>` + "\n" + `</pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("syntax highlighting json", func(t *testing.T) {
		doc, err := Run([]byte("```json\n{\"foo\": 123}\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre style="background-color:#fff">{&#34;foo&#34;: 123}` + "\n" + `</pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("heading consisting only of link uses link URL", func(t *testing.T) {
		doc, err := Run([]byte(`## [A](B)`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a"><a name="a" aria-hidden="true"></a><a href="B">A</a></h2>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("HTML", func(t *testing.T) {
		doc, err := Run([]byte(`<kbd>b</kbd>`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><kbd>b</kbd></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("relative URL in Markdown links and images", func(t *testing.T) {
		doc, err := Run([]byte("[a](b/c) ![a](b/c)"), Options{Base: &url.URL{Path: "/d/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><a href="/d/b/c">a</a> <img src="/d/b/c" alt="a" /></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
		}
	})
	t.Run("relative URL in HTML <a> and <img>", func(t *testing.T) {
		doc, err := Run([]byte(`<a href="b/c">z</a><img src="b/c">`), Options{Base: &url.URL{Path: "/d/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><a href="/d/b/c">z</a><img src="/d/b/c"></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("list", func(t *testing.T) {
		t.Run("bare items", func(t *testing.T) {
			doc, err := Run([]byte(`
- a
- b
- c`), Options{})
			if err != nil {
				t.Fatal(err)
			}
			want := `<ul>
<li>a</li>
<li>b</li>
<li>c</li>
</ul>` + "\n"
			if string(doc.HTML) != want {
				t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
			}
		})
		t.Run("nested items", func(t *testing.T) {
			// Not sure why a single extra blank line causes all list items to be wrapped in <p>,
			// but this is how GitHub works, too.
			doc, err := Run([]byte(`
- a

- b
- c`), Options{})
			if err != nil {
				t.Fatal(err)
			}
			want := `<ul>
<li><p>a</p></li>

<li><p>b</p></li>

<li><p>c</p></li>
</ul>` + "\n"
			if string(doc.HTML) != want {
				t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
			}
		})
	})
	t.Run("alerts", func(t *testing.T) {
		doc, err := Run([]byte(`> NOTE: **a**

x

> WARNING: **b**`), Options{Base: &url.URL{Path: "/d/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<aside class="note">
<p>NOTE: <strong>a</strong></p>
</aside>

<p>x</p>
<aside class="warning">

<p>WARNING: <strong>b</strong></p>
</aside>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
		}
	})
}

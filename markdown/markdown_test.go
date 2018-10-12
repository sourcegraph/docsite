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
	t.Run("heading anchor link", func(t *testing.T) {
		doc, err := Run([]byte(`## A ' B " C & D ? E`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a-b-c-d-e"><a name="a-b-c-d-e" class="anchor" href="#a-b-c-d-e" rel="nofollow" aria-hidden="true"></a>A &lsquo; B &ldquo; C &amp; D ? E</h2>` + "\n"
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

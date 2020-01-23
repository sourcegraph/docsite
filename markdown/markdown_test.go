package markdown

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strings"
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
	ctx := context.Background()
	t.Run("no metadata", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`# My title
Hello world github/linguist#1 **cool**, and #1!`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		check(t, *doc, Document{
			Title: "My title",
			HTML: []byte(`<h1 id="my-title"><a name="my-title" class="anchor" href="#my-title" rel="nofollow" aria-hidden="true" title="#my-title"></a>My title</h1>

<p>Hello world github/linguist#1 <strong>cool</strong>, and #1!</p>
`),
		})
	})
	t.Run("metadata", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`---
title: Metadata title
---

# Markdown title`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		check(t, *doc, Document{
			Meta:  Metadata{Title: "Metadata title"},
			Title: "Metadata title",
			HTML: []byte(`<h1 id="markdown-title"><a name="markdown-title" class="anchor" href="#markdown-title" rel="nofollow" aria-hidden="true" title="#markdown-title"></a>Markdown title</h1>
`),
		})
	})
}

func TestRenderer(t *testing.T) {
	ctx := context.Background()
	t.Run("table with `|`", func(t *testing.T) {
		doc, err := Run(ctx, []byte("a  |  b\n---|---\nc  | `\\|`"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<table>
<thead>
<tr>
<th>a</th>
<th>b</th>
</tr>
</thead>

<tbody>
<tr>
<td>c</td>
<td><code>|</code></td>
</tr>
</tbody>
</table>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  '%s'\nwant: '%s'", string(doc.HTML), want)
		}
	})
	t.Run("heading anchor link ignores special chars", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`## A ' B " C & D ? E`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a-b-c-d-e"><a name="a-b-c-d-e" class="anchor" href="#a-b-c-d-e" rel="nofollow" aria-hidden="true" title="#a-b-c-d-e"></a>A &lsquo; B &ldquo; C &amp; D ? E</h2>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("heading anchor link ignores markup", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`## [A](B) C`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a-c"><a name="a-c" class="anchor" href="#a-c" rel="nofollow" aria-hidden="true" title="#a-c"></a><a href="B">A</a> C</h2>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("disambiguates heading anchor", func(t *testing.T) {
		doc, err := Run(ctx, []byte("# A\n\n# A"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h1 id="a"><a name="a" class="anchor" href="#a" rel="nofollow" aria-hidden="true" title="#a"></a>A</h1>` + "\n\n" + `<h1 id="a-1"><a name="a-1" class="anchor" href="#a-1" rel="nofollow" aria-hidden="true" title="#a-1"></a>A</h1>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
		wantTree := []*SectionNode{
			{
				Title: "A", URL: "#a", Level: 1,
			},
			{
				Title: "A", URL: "#a-1", Level: 1,
			},
		}
		if !reflect.DeepEqual(doc.Tree, wantTree) {
			a, _ := json.MarshalIndent(doc.Tree, "", "  ")
			b, _ := json.MarshalIndent(wantTree, "", "  ")
			t.Errorf("\ngot:\n%s\n\nwant:\n%s", a, b)
		}
	})
	t.Run("explicit anchors", func(t *testing.T) {
		t.Run("heading", func(t *testing.T) {
			doc, err := Run(ctx, []byte(`# a {#b}`), Options{})
			if err != nil {
				t.Fatal(err)
			}
			want := `<h1 id="b"><a name="b" class="anchor" href="#b" rel="nofollow" aria-hidden="true" title="#b"></a>a</h1>` + "\n"
			if string(doc.HTML) != want {
				t.Errorf("got %q, want %q", string(doc.HTML), want)
			}
		})
		t.Run("inline", func(t *testing.T) {
			doc, err := Run(ctx, []byte(`a {#b} c`), Options{})
			if err != nil {
				t.Fatal(err)
			}
			want := `<p>a <span id="b" class="anchor-inline"></span><a href="#b" class="anchor-inline-link" title="#b"></a> c</p>` + "\n"
			if string(doc.HTML) != want {
				t.Errorf("got %q, want %q", string(doc.HTML), want)
			}
		})
	})
	t.Run("syntax highlighting go", func(t *testing.T) {
		doc, err := Run(ctx, []byte("```go\nvar foo struct{}\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre style=""><span style="color:#00f">var</span> foo <span style="color:#00f">struct</span>{}` + "\n" + `</pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("syntax highlighting typescript", func(t *testing.T) {
		doc, err := Run(ctx, []byte("```typescript\nconst foo = 'bar'\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre style=""><span style="color:#00f">const</span> foo = <span style="color:#a31515">&#39;bar&#39;</span>` + "\n" + `</pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("syntax highlighting json", func(t *testing.T) {
		doc, err := Run(ctx, []byte("```json\n{\"foo\": 123}\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre style="">{&#34;foo&#34;: 123}` + "\n" + `</pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("heading consisting only of link uses link URL", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`## [A](B)`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a"><a name="a" aria-hidden="true"></a><a href="B">A</a></h2>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("HTML", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`<kbd>b</kbd>`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><kbd>b</kbd></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("HTML div", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`<div class="foo">b</div>`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><div class="foo">b</div></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("relative URL in Markdown links and images", func(t *testing.T) {
		doc, err := Run(ctx, []byte("[a](b/c) ![a](b/c)"), Options{Base: &url.URL{Path: "/d/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><a href="/d/b/c">a</a> <img src="/d/b/c" alt="a" /></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
		}
	})
	t.Run("relative URL in HTML <a> and <img>", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`<a href="b/c">z</a><img src="b/c">`), Options{Base: &url.URL{Path: "/d/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><a href="/d/b/c">z</a><img src="/d/b/c"></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("anchor link", func(t *testing.T) {
		doc, err := Run(ctx, []byte("[a](#b) <a href='#c'>d</a>"), Options{Base: &url.URL{Path: "/d/e"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><a href="#b">a</a> <a href="#c">d</a></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
		}
	})
	t.Run("list", func(t *testing.T) {
		t.Run("bare items", func(t *testing.T) {
			doc, err := Run(ctx, []byte(`
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
			doc, err := Run(ctx, []byte(`
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
	t.Run("empty blockquote", func(t *testing.T) {
		doc, err := Run(ctx, []byte("> `a`"), Options{Base: &url.URL{Path: "/d/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<blockquote>
<p><code>a</code></p>
</blockquote>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
		}
	})
	t.Run("alerts", func(t *testing.T) {
		doc, err := Run(ctx, []byte(`> NOTE: **a**

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
	t.Run("markdown-func", func(t *testing.T) {
		t.Run("start-end tag", func(t *testing.T) {
			doc, err := Run(ctx, []byte(`<div markdown-func=x x:a=b>z</div>
`), Options{
				Base: &url.URL{},
				Funcs: FuncMap{
					"x": func(ctx context.Context, info FuncInfo, args map[string]string) (string, error) {
						b, err := json.Marshal(args)
						return info.Version + " " + string(b), err
					},
				},
				FuncInfo: FuncInfo{Version: "v"},
			})
			if err != nil {
				t.Fatal(err)
			}
			want := `v {"a":"b"}` + "\n"
			if string(doc.HTML) != want {
				t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
			}
		})
		t.Run("self-closing tag", func(t *testing.T) {
			doc, err := Run(ctx, []byte(`<div markdown-func=x x:a=b />`), Options{
				Base: &url.URL{},
				Funcs: FuncMap{
					"x": func(ctx context.Context, info FuncInfo, args map[string]string) (string, error) {
						b, err := json.Marshal(args)
						return string(b), err
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			want := `<p>{"a":"b"}</p>` + "\n"
			if string(doc.HTML) != want {
				t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
			}
		})
		t.Run("never closed", func(t *testing.T) {
			_, err := Run(ctx, []byte(`<div markdown-func=x x:a=b>z
`), Options{
				Base: &url.URL{},
				Funcs: FuncMap{
					"x": func(ctx context.Context, info FuncInfo, args map[string]string) (string, error) {
						return "", nil
					},
				},
			})
			if want := "tag for Markdown function \"x\" is never closed"; err == nil || err.Error() != want {
				t.Fatalf("got error %v, want %q", err, want)
			}
		})
		t.Run("error", func(t *testing.T) {
			_, err := Run(ctx, []byte(`<div markdown-func=x />`), Options{
				Base: &url.URL{},
				Funcs: FuncMap{
					"x": func(ctx context.Context, info FuncInfo, args map[string]string) (string, error) {
						return "", errors.New("z")
					},
				},
			})
			if want := "error in Markdown function \"x\": z"; err == nil || err.Error() != want {
				t.Fatalf("got error %v, want %q", err, want)
			}
		})
		t.Run("panic", func(t *testing.T) {
			_, err := Run(ctx, []byte(`<div markdown-func=x />`), Options{
				Base: &url.URL{},
				Funcs: FuncMap{
					"x": func(ctx context.Context, info FuncInfo, args map[string]string) (string, error) {
						panic("z")
					},
				},
			})
			if want := "error in Markdown function \"x\": z"; err == nil || err.Error() != want {
				t.Fatalf("got error %v, want %q", err, want)
			}
		})
		t.Run("nonexistent", func(t *testing.T) {
			_, err := Run(ctx, []byte(`<div markdown-func=x>b</div>`), Options{Base: &url.URL{}})
			if want := "Markdown function \"x\" is not defined"; err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("got error %v, want %q", err, want)
			}
		})
	})
}

func TestJoinBytesAsText(t *testing.T) {
	tests := map[string]struct {
		parts []string
		want  string
	}{
		"adjacent words": {
			parts: []string{"a", "b"},
			want:  "a b",
		},
		"adjacent sentences": {
			parts: []string{"a.", "b."},
			want:  "a. b.",
		},
		"end of sentence": {
			parts: []string{"a", ". b."},
			want:  "a. b.",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			parts := make([][]byte, len(test.parts))
			for i, part := range test.parts {
				parts[i] = []byte(part)
			}
			got := joinBytesAsText(parts)
			if string(got) != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestGetTitle(t *testing.T) {
	tests := map[string]string{
		"# h":               "h",
		"# h\n\n# i":        "h",
		"<!-- a -->\n# h":   "h",
		"<!-- a --> \n# h":  "h",
		"<!-- a -->\n\n# h": "h",
		"a\n# h":            "",
	}
	for input, wantTitle := range tests {
		t.Run(input, func(t *testing.T) {
			ast := NewParser(nil).Parse([]byte(input))
			title := GetTitle(ast)
			if title != wantTitle {
				t.Errorf("got title %q, want %q", title, wantTitle)
			}
		})
	}
}

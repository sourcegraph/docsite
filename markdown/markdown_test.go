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

	"github.com/yuin/goldmark/text"
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
			HTML: []byte(`<h1 id="my-title"><a name="my-title" class="anchor" href="#my-title" rel="nofollow" aria-hidden="true" title="#my-title"></a>My title</h1>
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
			HTML: []byte(`<h1 id="markdown-title"><a name="markdown-title" class="anchor" href="#markdown-title" rel="nofollow" aria-hidden="true" title="#markdown-title"></a>Markdown title</h1>
`),
		})
	})
}

func TestRenderer(t *testing.T) {
	t.Run("table with `|`", func(t *testing.T) {
		doc, err := Run([]byte("a  |  b\n---|---\nc  | `\\|`"), Options{})
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
		doc, err := Run([]byte(`## A ' B " C & D ? E`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a-b-c-d-e"><a name="a-b-c-d-e" class="anchor" href="#a-b-c-d-e" rel="nofollow" aria-hidden="true" title="#a-b-c-d-e"></a>A ' B " C & D ? E</h2>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("heading anchor link ignores markup", func(t *testing.T) {
		doc, err := Run([]byte(`## [A](B) C`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a-b-c"><a name="a-b-c" class="anchor" href="#a-b-c" rel="nofollow" aria-hidden="true" title="#a-b-c"></a><a href="B">A</a> C</h2>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("disambiguate heading anchor", func(t *testing.T) {
		doc, err := Run([]byte("# A\n\n# A"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h1 id="a"><a name="a" class="anchor" href="#a" rel="nofollow" aria-hidden="true" title="#a"></a>A</h1>
<h1 id="a-1"><a name="a-1" class="anchor" href="#a-1" rel="nofollow" aria-hidden="true" title="#a-1"></a>A</h1>
`
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
			doc, err := Run([]byte(`# a {#b}`), Options{})
			if err != nil {
				t.Fatal(err)
			}
			want := `<h1 id="b"><a name="b" class="anchor" href="#b" rel="nofollow" aria-hidden="true" title="#b"></a>a
</h1>
`
			if string(doc.HTML) != want {
				t.Errorf("got %q, want %q", string(doc.HTML), want)
			}
		})
		t.Run("inline", func(t *testing.T) {
			doc, err := Run([]byte(`a {#b} c`), Options{})
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
		doc, err := Run([]byte("```go\nvar foo struct{}\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre class="chroma go"><span class="line"><span class="cl"><span class="kd">var</span> <span class="nx">foo</span> <span class="kd">struct</span><span class="p">{}</span>
</span></span></pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("syntax highlighting typescript", func(t *testing.T) {
		doc, err := Run([]byte("```typescript\nconst foo = 'bar'\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre class="chroma typescript"><span class="line"><span class="cl"><span class="kr">const</span> <span class="nx">foo</span> <span class="o">=</span> <span class="s1">&#39;bar&#39;</span>
</span></span></pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("syntax highlighting json", func(t *testing.T) {
		doc, err := Run([]byte("```json\n{\"foo\": 123}\n```"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<pre class="chroma json"><span class="line"><span class="cl"><span class="p">{</span><span class="nt">&#34;foo&#34;</span><span class="p">:</span> <span class="mi">123</span><span class="p">}</span>
</span></span></pre>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("heading consisting only of link uses link URL", func(t *testing.T) {
		doc, err := Run([]byte(`## [A](B)`), Options{})
		if err != nil {
			t.Fatal(err)
		}
		want := `<h2 id="a-b"><a name="a-b" aria-hidden="true"></a><a href="B">A</a></h2>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("HTML", func(t *testing.T) {
		doc, err := Run([]byte(`<kbd>b</kbd>`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><kbd>b</kbd></p>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("HTML div", func(t *testing.T) {
		doc, err := Run([]byte(`<div class="foo">b</div>`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<div class="foo">b</div>`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("month date", func(t *testing.T) {
		doc, err := Run([]byte(`To be completed by 2021-06.`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p>To be completed by 2021-06.</p>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("full date", func(t *testing.T) {
		doc, err := Run([]byte(`To be completed by 2021-06-20.`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p>To be completed by 2021-06-20.</p>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("full datetime", func(t *testing.T) {
		doc, err := Run([]byte(`We meet up at 2021-06-20 10:00Z.`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p>We meet up at 2021-06-20 10:00Z.</p>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("fiscal year long", func(t *testing.T) {
		doc, err := Run([]byte(`Plan for FY2022:`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p>Plan for <time datetime="2021-02-01" data-is-start-of-interval="true">FY2022</time>:</p>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("fiscal year short", func(t *testing.T) {
		doc, err := Run([]byte(`Plan for FY22:`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p>Plan for <time datetime="2021-02-01" data-is-start-of-interval="true">FY22</time>:</p>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("fiscal quarter", func(t *testing.T) {
		doc, err := Run([]byte(`Plan for FY22Q2:`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p>Plan for <time datetime="2021-05-01" data-is-start-of-interval="true">FY22Q2</time>:</p>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("fiscal quarter only", func(t *testing.T) {
		doc, err := Run([]byte(`Plan for FQ2:`), Options{Base: &url.URL{}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p>Plan for <time datetime="05-01" data-is-start-of-interval="true">FQ2</time>:</p>
`
		if string(doc.HTML) != want {
			t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
		}
	})
	t.Run("relative URL in Markdown links and images", func(t *testing.T) {
		doc, err := Run([]byte("[a](b/c) ![a](b/c)"), Options{Base: &url.URL{Path: "/d/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><a href="/d/b/c">a</a> <img src="/d/b/c" alt="a"></p>
`
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
	t.Run("anchor link", func(t *testing.T) {
		doc, err := Run([]byte("[a](#b) <a href='#c'>d</a>"), Options{Base: &url.URL{Path: "/d/e"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<p><a href="#b">a</a> <a href="#c">d</a></p>` + "\n"
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
		}
	})
	t.Run("inline javascript script tag", func(t *testing.T) {
		doc, err := Run([]byte("<script>'a'</script>\n\na"), Options{Base: &url.URL{Path: "/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := "<script>'a'</script>\n<p>a</p>\n"
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
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
<li>
<p>a</p>
</li>
<li>
<p>b</p>
</li>
<li>
<p>c</p>
</li>
</ul>
`
			if string(doc.HTML) != want {
				t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
			}
		})
	})
	t.Run("empty blockquote", func(t *testing.T) {
		doc, err := Run([]byte("> `a`"), Options{Base: &url.URL{Path: "/d/"}})
		if err != nil {
			t.Fatal(err)
		}
		want := `<blockquote>
<p><code>a</code></p>
</blockquote>
`
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
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
</aside>
`
		if string(doc.HTML) != want {
			t.Errorf("got %q, want %q", string(doc.HTML), want)
		}
	})
	t.Run("markdown-func", func(t *testing.T) {
		t.Run("start-end tag", func(t *testing.T) {
			doc, err := Run([]byte(`<div markdown-func=x x:a=b>z</div>
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
			doc, err := Run([]byte(`<div markdown-func=x x:a=b />`), Options{
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
			want := `{"a":"b"}`
			if string(doc.HTML) != want {
				t.Errorf("\ngot:  %s\nwant: %s", string(doc.HTML), want)
			}
		})
		t.Run("never closed", func(t *testing.T) {
			_, err := Run([]byte(`<div markdown-func=x x:a=b>z
`), Options{
				Base: &url.URL{},
				Funcs: FuncMap{
					"x": func(ctx context.Context, info FuncInfo, args map[string]string) (string, error) {
						return "", nil
					},
				},
			})
			if want := "render: tag for Markdown function \"x\" is never closed"; err == nil || err.Error() != want {
				t.Fatalf("got error %v, want %q", err, want)
			}
		})
		t.Run("error", func(t *testing.T) {
			_, err := Run([]byte(`<div markdown-func=x />`), Options{
				Base: &url.URL{},
				Funcs: FuncMap{
					"x": func(ctx context.Context, info FuncInfo, args map[string]string) (string, error) {
						return "", errors.New("z")
					},
				},
			})
			if want := "render: error in Markdown function \"x\": z"; err == nil || err.Error() != want {
				t.Fatalf("got error %v, want %q", err, want)
			}
		})
		t.Run("panic", func(t *testing.T) {
			_, err := Run([]byte(`<div markdown-func=x />`), Options{
				Base: &url.URL{},
				Funcs: FuncMap{
					"x": func(ctx context.Context, info FuncInfo, args map[string]string) (string, error) {
						panic("z")
					},
				},
			})
			if want := "render: error in Markdown function \"x\": z"; err == nil || err.Error() != want {
				t.Fatalf("got error %v, want %q", err, want)
			}
		})
		t.Run("nonexistent", func(t *testing.T) {
			_, err := Run([]byte(`<div markdown-func=x>b</div>`), Options{Base: &url.URL{}})
			if want := "Markdown function \"x\" is not defined"; err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("got error %v, want %q", err, want)
			}
		})
	})
}

func TestGetTitle(t *testing.T) {
	tests := map[string]string{
		"# h":               "h",
		"# h\n\n# i":        "h",
		"<!-- a -->\n# h":   "h",
		"<!-- a --> \n# h":  "h",
		"<!-- a -->\n\n# h": "h",
		"a\n# h":            "h",
	}
	for input, wantTitle := range tests {
		t.Run(input, func(t *testing.T) {
			mdAST := New(Options{}).Parser().Parse(text.NewReader([]byte(input)))
			title := GetTitle(mdAST, []byte(input))
			if title != wantTitle {
				t.Errorf("got title %q, want %q", title, wantTitle)
			}
		})
	}
}

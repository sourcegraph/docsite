package docsite

import (
	"context"
	"net/url"
	"os"
	"testing"

	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

func TestSite_ResolveContentPage(t *testing.T) {
	ctx := context.Background()
	site := Site{
		Content: versionedFileSystem{
			"": httpfs.New(mapfs.New(map[string]string{
				"index.md":     "z",
				"a/b/index.md": "e",
				"a/b/c.md":     "d",
			})),
		},
		Base: &url.URL{Path: "/"},
	}

	t.Run("page", func(t *testing.T) {
		page, err := site.ResolveContentPage(ctx, "", "a/b/c")
		if err != nil {
			t.Fatal(err)
		}
		if want := "a/b/c"; page.Path != want {
			t.Errorf("got path %q, want %q", page.Path, want)
		}
		if want := "a/b/c.md"; page.FilePath != want {
			t.Errorf("got file path %q, want %q", page.FilePath, want)
		}
		if want := "d"; string(page.Data) != want {
			t.Errorf("got data %q, want %q", page.Data, want)
		}
	})

	t.Run("root", func(t *testing.T) {
		page, err := site.ResolveContentPage(ctx, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if want := ""; page.Path != want {
			t.Errorf("got path %q, want %q", page.Path, want)
		}
		if want := "index.md"; page.FilePath != want {
			t.Errorf("got file path %q, want %q", page.FilePath, want)
		}
		if want := "z"; string(page.Data) != want {
			t.Errorf("got data %q, want %q", page.Data, want)
		}
	})

	t.Run("resolved to different path", func(t *testing.T) {
		page, err := site.ResolveContentPage(ctx, "", "a/b/index")
		if err != nil {
			t.Fatal(err)
		}
		if want := "a/b"; page.Path != want {
			t.Errorf("got path %q, want %q", page.Path, want)
		}
		if want := "a/b/index.md"; page.FilePath != want {
			t.Errorf("got file path %q, want %q", page.FilePath, want)
		}
		if want := "e"; string(page.Data) != want {
			t.Errorf("got data %q, want %q", page.Data, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := site.ResolveContentPage(ctx, "", "not/found"); !os.IsNotExist(err) {
			t.Errorf("got error %v, want os.IsNotExist(err) == true", err)
		}
	})
}

func TestSite_RenderContentPage(t *testing.T) {
	site := Site{
		Templates: httpfs.New(mapfs.New(map[string]string{
			"template.html": `
{{define "root" -}}
{{range .Breadcrumbs}}{{.Label}} ({{.URL}}){{if not .IsActive}} / {{end}}{{end}}
{{markdown .}}
{{- end}}`,
		})),
		Base: &url.URL{Path: "/"},
	}

	t.Run("page", func(t *testing.T) {
		page, err := site.newContentPage("a/b/c.md", []byte("d [z](../e/f/index.md)"), "")
		if err != nil {
			t.Fatal(err)
		}
		data, err := site.RenderContentPage(page)
		if err != nil {
			t.Fatal(err)
		}
		if want := `Documentation (/) / a (/a) / b (/a/b) / c (/a/b/c)
<p>d <a href="/a/e/f">z</a></p>
`; string(data) != want {
			t.Errorf("got %q, want %q", data, want)
		}
	})

	t.Run("index", func(t *testing.T) {
		page, err := site.newContentPage("a/b/index.md", []byte("d [z](../e/f/index.md)"), "")
		if err != nil {
			t.Fatal(err)
		}
		data, err := site.RenderContentPage(page)
		if err != nil {
			t.Fatal(err)
		}
		if want := `Documentation (/) / a (/a) / b (/a/b)
<p>d <a href="/a/e/f">z</a></p>
`; string(data) != want {
			t.Errorf("got %q, want %q", data, want)
		}
	})
}

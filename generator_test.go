package docsite

import (
	"testing"

	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

func TestGenerator(t *testing.T) {
	g := Generator{
		Sources: httpfs.New(mapfs.New(map[string]string{
			"a/b/c.md": "d",
		})),
		Templates: httpfs.New(mapfs.New(map[string]string{
			"template.html": `
{{define "root" -}}
{{range .Breadcrumbs}}{{.Label}} ({{.URL}}){{if not .IsActive}} / {{end}}{{end}}
{{markdown .}}
{{- end}}`,
		})),
		AssetsURLPathPrefix: "/assets/",
	}

	data, err := g.Generate("a/b/c")
	if err != nil {
		t.Fatal(err)
	}
	if want := `Documentation (/) / a (/a) / b (/a/b) / c (/a/b/c)
<p>d</p>
`; string(data) != want {
		t.Errorf("got %q, want %q", data, want)
	}
}

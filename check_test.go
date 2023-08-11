package docsite

import (
	"context"
	"net/url"
	"reflect"
	"regexp"
	"testing"

	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

func TestCheck(t *testing.T) {
	tests := map[string]struct {
		pages        map[string]string
		wantProblems []string
	}{
		"valid links": {
			pages: map[string]string{
				"index.md":                           "[a](index.md) [b](b/index.md)",
				"b/index.md":                         "[a](../index.md) [b](index.md)",
				"_resources/templates/root.html":     "{{markdown .Content}}",
				"_resources/templates/document.html": "{{markdown .Content}}",
			},
			wantProblems: nil,
		},
		"non-relative link path": {
			pages: map[string]string{
				"index.md":                           "[a](/index.md)",
				"_resources/templates/root.html":     "{{markdown .Content}}",
				"_resources/templates/document.html": "{{markdown .Content}}",
			},
			wantProblems: []string{"index.md: must use relative, not absolute, link to /index.md"},
		},
		"scheme-relative link": {
			pages: map[string]string{
				"index.md":                           "[a](//example.com/a)",
				"_resources/templates/root.html":     "{{markdown .Content}}",
				"_resources/templates/document.html": "{{markdown .Content}}",
			},
			wantProblems: nil,
		},
		"broken link": {
			pages: map[string]string{
				"index.md":                           "[x](x.md)",
				"_resources/templates/root.html":     "{{markdown .Content}}",
				"_resources/templates/document.html": "{{markdown .Content}}",
			},
			wantProblems: []string{"index.md: broken link to /x"},
		},
		"link to equivalent path not .md file": {
			pages: map[string]string{
				"index.md": "[a](a) [a](a.md)", "a.md": "",
				"_resources/templates/root.html":     "{{markdown .Content}}",
				"_resources/templates/document.html": "{{markdown .Content}}",
			},
			wantProblems: []string{"index.md: must link to .md file, not a"},
		},
		"disconnected page": {
			pages: map[string]string{
				"x.md":                               "[x](x.md)",
				"_resources/templates/root.html":     "{{markdown .Content}}",
				"_resources/templates/document.html": "{{markdown .Content}}",
			},
			wantProblems: []string{"x.md: disconnected page (no inlinks from other pages)"},
		},
		"ignore disconnected page check": {
			pages: map[string]string{
				"x.md":                               "---\nignoreDisconnectedPageCheck: true\n---\n\n[x](x.md)",
				"_resources/templates/root.html":     "{{markdown .Content}}",
				"_resources/templates/document.html": "{{markdown .Content}}",
			},
			wantProblems: nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			site := Site{
				Content: versionedFileSystem{
					"":                     httpfs.New(mapfs.New(test.pages)),
					"_resources/templates": httpfs.New(mapfs.New(map[string]string{"document.html": "{{markdown .Content}}"})),
				},
				// Templates:             httpfs.New(mapfs.New(map[string]string{"document.html": "{{markdown .Content}}"})),
				Base:                  &url.URL{Path: "/"},
				CheckIgnoreURLPattern: regexp.MustCompile(`^//`),
			}
			problems, err := site.Check(ctx, "")
			if err != nil {
				t.Fatal(err)
			}
			problemsSet := toSet(problems)
			wantProblemsSet := toSet(test.wantProblems)
			if !reflect.DeepEqual(problemsSet, wantProblemsSet) {
				t.Errorf("got problems %v, want %v", problemsSet, wantProblemsSet)
			}
		})
	}
}

func toSet(s []string) map[string]struct{} {
	m := make(map[string]struct{}, len(s))
	for _, v := range s {
		m[v] = struct{}{}
	}
	return m
}

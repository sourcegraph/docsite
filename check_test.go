package docsite

import (
	"context"
	"net/url"
	"reflect"
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
				"a.md":       "[a](a.md) [b](b/index.md)",
				"b/index.md": "[a](../a.md) [b](index.md)",
			},
			wantProblems: nil,
		},
		"non-relative link path": {
			pages:        map[string]string{"a.md": "[a](/a.md)"},
			wantProblems: []string{"a.md: must use relative, not absolute, link to /a.md"},
		},
		"broken link": {
			pages:        map[string]string{"a.md": "[b](b.md)"},
			wantProblems: []string{"a.md: broken link to /b"},
		},
		"link to equivalent path not .md file": {
			pages:        map[string]string{"a.md": "[a](a)"},
			wantProblems: []string{"a.md: must link to .md file, not a"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			site := Site{
				Content: versionedFileSystem{
					"": httpfs.New(mapfs.New(test.pages)),
				},
				Templates: httpfs.New(mapfs.New(map[string]string{"doc.html": "{{markdown .Content}}"})),
				Base:      &url.URL{Path: "/"},
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

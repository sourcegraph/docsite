package docsite

import (
	"reflect"
	"testing"
)

func TestContentFilePathToPath(t *testing.T) {
	tests := map[string]string{
		"index.md":   "",
		"a.md":       "a",
		"a/b.md":     "a/b",
		"a/index.md": "a",
	}
	for filePath, wantPath := range tests {
		path := contentFilePathToPath(filePath)
		if path != wantPath {
			t.Errorf("%s: got %q, want %q", filePath, path, wantPath)
		}
	}
}

func TestMakeBreadcrumbEntries(t *testing.T) {
	tests := map[string][]breadcrumbEntry{
		"a/b/c": []breadcrumbEntry{
			{Label: "Documentation", URL: "/", IsActive: false},
			{Label: "a", URL: "/a", IsActive: false},
			{Label: "b", URL: "/a/b", IsActive: false},
			{Label: "c", URL: "/a/b/c", IsActive: true},
		},
		"a": []breadcrumbEntry{
			{Label: "Documentation", URL: "/", IsActive: false},
			{Label: "a", URL: "/a", IsActive: true},
		},
		"": nil,
	}
	for path, want := range tests {
		t.Run(path, func(t *testing.T) {
			got := makeBreadcrumbEntries(path)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

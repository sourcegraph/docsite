package docsite

import "testing"

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

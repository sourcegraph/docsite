package docsite

import (
	"context"
	"net/http"
	"os"
	"reflect"
	"sort"
	"testing"

	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

func TestWalkFileSystem(t *testing.T) {
	wantAllPaths := []string{
		"a/b.md",
		"a/c/d.md",
		"e.md",
		"f/g.md",
		"f/h.md",
	}
	files := make(map[string]string, len(wantAllPaths))
	for _, path := range wantAllPaths {
		files[path] = ""
	}
	fs := httpfs.New(mapfs.New(files))

	var allPaths []string
	collect := func(path string) error {
		allPaths = append(allPaths, path)
		return nil
	}
	if err := WalkFileSystem(fs, collect); err != nil {
		t.Fatal(err)
	}
	sort.Strings(allPaths)
	if !reflect.DeepEqual(allPaths, wantAllPaths) {
		t.Errorf("got paths %v, want %v", allPaths, wantAllPaths)
	}
}

type versionedFileSystem map[string]http.FileSystem

func (vfs versionedFileSystem) OpenVersion(_ context.Context, version string) (http.FileSystem, error) {
	fs, ok := vfs[version]
	if !ok {
		return nil, &os.PathError{Op: "OpenVersion", Path: version, Err: os.ErrNotExist}
	}
	return fs, nil
}

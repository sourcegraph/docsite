package main

import (
	"reflect"
	"sort"
	"testing"

	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

func TestWalkFileSystem(t *testing.T) {
	wantAllPaths := []string{
		"/a/b.md",
		"/a/c/d.md",
		"/e.md",
		"/f/g.md",
		"/f/h.md",
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
	if err := walkFileSystem(fs, collect); err != nil {
		t.Fatal(err)
	}
	sort.Strings(allPaths)
	if !reflect.DeepEqual(allPaths, wantAllPaths) {
		t.Errorf("got paths %v, want %v", allPaths, wantAllPaths)
	}
}

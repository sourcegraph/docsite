package docsite

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// WalkFileSystem walks a file system and calls walkFn for each file that passes filterFn.
func WalkFileSystem(fs http.FileSystem, filterFn func(path string) bool, walkFn func(path string) error) error {
	path := "/"
	root, err := fs.Open(path)
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("open walk root %s", path))
	}
	fi, err := root.Stat()
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("stat walk root %s", path))
	}

	type queueItem struct {
		path string
		fi   os.FileInfo
	}
	queue := []queueItem{{path: path, fi: fi}}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if item.fi.Mode().IsDir() {
			if strings.HasPrefix(item.fi.Name(), ".") {
				continue // skip dot-dirs
			}
			dir, err := fs.Open(item.path)
			if err != nil {
				return errors.WithMessage(err, fmt.Sprintf("open %s", item.path))
			}
			entries, err := dir.Readdir(-1)
			if err != nil {
				return errors.WithMessage(err, fmt.Sprintf("readdir %s", item.path))
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
			for _, e := range entries {
				queue = append(queue, queueItem{path: filepath.Join(item.path, e.Name()), fi: e})
			}
		} else if filterFn(item.path) {
			if err := walkFn(strings.TrimPrefix(item.path, "/")); err != nil {
				return errors.WithMessage(err, fmt.Sprintf("walk %s", item.path))
			}
		}
	}
	return nil
}

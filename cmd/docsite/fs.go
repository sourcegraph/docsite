package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

func skipFile(path string) bool {
	return filepath.Ext(path) != ".md"
}

func walkFileSystem(fs http.FileSystem, walkFn func(path string) error) error {
	path := "/"
	root, err := fs.Open(path)
	if err != nil {
		return err
	}
	fi, err := root.Stat()
	if err != nil {
		return err
	}

	type queueItem struct {
		path string
		fi   os.FileInfo
	}
	queue := []queueItem{{path: path, fi: fi}}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		switch {
		case item.fi.Mode().IsDir(): // dir
			if strings.HasPrefix(item.fi.Name(), ".") {
				continue // skip dot-dirs
			}
			dir, err := fs.Open(item.path)
			if err != nil {
				return err
			}
			entries, err := dir.Readdir(-1)
			if err != nil {
				return err
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
			for _, e := range entries {
				queue = append(queue, queueItem{path: filepath.Join(item.path, e.Name()), fi: e})
			}
		case item.fi.Mode().IsRegular(): // file
			if err := walkFn(strings.TrimPrefix(item.path, "/")); err != nil {
				return errors.WithMessage(err, fmt.Sprintf("walk %s", item.path))
			}
		default:
			return fmt.Errorf("file %s has unsupported mode %o (symlinks and other special files are not supported)", item.path, item.fi.Mode())
		}
	}
	return nil
}

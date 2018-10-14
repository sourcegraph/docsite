package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

func init() {
	flagSet := flag.NewFlagSet("generate", flag.ExitOnError)
	var (
		outDir = flagSet.String("out", "out", "path to output `dir` where .html files are written")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)
		gen := generatorFromFlags()
		if err := os.MkdirAll(*outDir, 0700); err != nil {
			return err
		}
		generateFile := func(path string) error {
			if filepath.Ext(path) != ".md" {
				return nil // nothing to do
			}
			data, err := gen.Generate(path, true)
			if err != nil {
				return err
			}
			return ioutil.WriteFile(filepath.Join(*outDir, path+".html"), data, 0600)
		}
		if err := walkFileSystem(gen.Sources, generateFile); err != nil {
			return err
		}
		log.Printf("# Wrote site files to %s", *outDir)
		return nil
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "write all output .html files for site",
		LongDescription:  "The generate subcommand generates all output .html files for the site and writes them to a directory.",
		aliases:          []string{"gen"},
		handler:          handler,
	})
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

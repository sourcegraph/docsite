package docsite

import (
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/docsite/markdown"
)

// resolveAndReadAll resolves a URL path to a file path, adding a file extension (.md) and a
// directory index filename as needed. It also returns the file content.
func resolveAndReadAll(fs http.FileSystem, path string) (filePath string, data []byte, err error) {
	filePath = path + ".md"
	data, err = ReadFile(fs, filePath)
	if isDir(fs, filePath) || (os.IsNotExist(err) && !strings.HasSuffix(path, string(os.PathSeparator)+"index")) {
		// Try looking up the path as a directory and reading its index file (index.md).
		return resolveAndReadAll(fs, filepath.Join(path, "index"))
	}
	return filePath, data, err
}

func isDir(fs http.FileSystem, path string) bool {
	f, err := fs.Open(path)
	if err != nil {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode().IsDir()
}

type sourceFile struct {
	FilePath    string
	Doc         markdown.Document
	Breadcrumbs []breadcrumbEntry
}

type breadcrumbEntry struct {
	Label    string
	URL      string
	IsActive bool
}

func makeBreadcrumbEntries(path string) []breadcrumbEntry {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	entries := make([]breadcrumbEntry, len(parts)+1)
	entries[0] = breadcrumbEntry{
		Label: "Documentation",
		URL:   "/",
	}
	for i, part := range parts {
		entries[i+1] = breadcrumbEntry{
			Label:    part,
			URL:      "/" + pathpkg.Join(parts[:i+1]...),
			IsActive: i == len(parts)-1,
		}
	}
	return entries
}

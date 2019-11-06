package docsite

import (
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/docsite/markdown"
)

// ContentPage represents a Markdown-formatted documentation page. To create a ContentPage, use one of the Site methods.
type ContentPage struct {
	Path        string            // the canonical URL path (without ".md" or "/index.md")
	FilePath    string            // the filename on disk
	Data        []byte            // the page's file contents
	Doc         markdown.Document // the Markdown doc
	Breadcrumbs []breadcrumbEntry // ancestor breadcrumb for this page
}

func contentFilePathToPath(filePath string) string {
	path := strings.TrimSuffix(filePath, ".md")
	if path == "index" {
		return ""
	}
	return strings.TrimSuffix(path, "/index")
}

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
	defer f.Close()
	fi, err := f.Stat()
	return err == nil && fi.Mode().IsDir()
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

func isContentPage(path string) bool {
	return filepath.Ext(path) == ".md"
}

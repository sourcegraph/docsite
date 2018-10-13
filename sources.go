package docsite

import (
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

// resolveAndReadAll resolves a URL path to a file path, adding a file extension (.md) and a
// directory index filename as needed. It also returns the file content.
func resolveAndReadAll(fs http.FileSystem, path string) (filePath string, data []byte, err error) {
	if path == "" {
		// Special-case: the top-level index file is README.md not index.md.
		path = "README"
	}

	filePath = path + ".md"
	data, err = readFile(fs, filePath)
	if os.IsNotExist(err) && !strings.HasSuffix(path, string(os.PathSeparator)+"index") {
		// Try looking up the path as a directory and reading its index file (index.md).
		return resolveAndReadAll(fs, filepath.Join(path, "index"))
	}
	return filePath, data, err
}

type sourceFile struct {
	FilePath    string
	Data        []byte
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

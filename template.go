package docsite

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/sourcegraph/docsite/markdown"
)

const (
	rootTemplateName     = "root"
	documentTemplateName = "document"
	searchTemplateName   = "search"
)

func (s *Site) getTemplate(templatesVFS VersionedFileSystem, name, contentVersion string, extraFuncs template.FuncMap) (*template.Template, error) {

	templatesFS, err := templatesVFS.OpenVersion(context.Background(), contentVersion)
	if err != nil {
		return nil, err
	}

	readFile := func(fs http.FileSystem, path string) ([]byte, error) {
		f, err := fs.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	tmpl := template.New(rootTemplateName)
	tmpl.Funcs(template.FuncMap{
		"asset": func(path string) string {
			assetUrl := s.AssetsBase.ResolveReference(&url.URL{Path: path}).String()
			// fs, err := s.Content.OpenVersion(context.Background(), version)
			// if err != nil {
			// 	return assetUrl
			// }

			return assetUrl
		},
		"assetsFromVersion": func(version, path string) string {
			assetUrl := s.AssetsBase.ResolveReference(&url.URL{Path: path, RawQuery: version}).String()
			fmt.Println("assetUrl lalala: ", assetUrl)
			fmt.Println("path & version: ", path, version)
			return assetUrl
		},
		"isVersioned": func(version string) bool {
			fmt.Println("isVersioned version: ", version)
			return version != ""
		},
		"contentFileExists": func(version, path string) bool {
			fmt.Println("how come version shows", version)
			fs, err := s.Content.OpenVersion(context.Background(), version)
			if err != nil {
				return false
			}
			f, err := fs.Open(path)
			// Treat all errors as "not-exists".
			if f != nil {
				f.Close()
			}
			return err == nil
		},
		"renderMarkdownContentFile": func(version, path string) (template.HTML, error) {
			fs, err := s.Content.OpenVersion(context.Background(), version)
			if err != nil {
				return "", err
			}
			data, err := readFile(fs, path)
			if err != nil {
				return "", err
			}
			doc, err := markdown.Run(data, s.markdownOptions(path, version))
			if err != nil {
				return "", err
			}
			return template.HTML(doc.HTML), nil
		},
		"subtract":   func(a, b int) int { return a - b },
		"replace":    strings.Replace,
		"trimPrefix": strings.TrimPrefix,
		"contains":   strings.Contains,
		"hasRootURL": func() bool {
			return s.Root != nil
		},
		"absURL": func(path string) string {
			if s.Root != nil {
				url := *s.Root
				url.Path = path
				return url.String()
			}
			return path
		},
	})
	tmpl.Funcs(extraFuncs)

	// Read root and named template files.
	names := []string{rootTemplateName, name}
	for _, name := range names {
		path := "/" + name + ".html"
		data, err := ReadFile(templatesFS, path)
		fmt.Println("path: ", path)
		if name == rootTemplateName && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("read template %s", path))
		}
		if _, err := tmpl.Parse(string(data)); err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("parse template %s", path))
		}
	}
	return tmpl, nil
}

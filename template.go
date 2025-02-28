package docsite

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
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
	metaRobots           = `<meta name="robots" content="noindex,nofollow">`
	metaProperty         = `<meta property="og:locale" content="en_EN">`
)

func patchTemplateForSEO(data []byte) []byte {
	if bytes.Contains(data, []byte(metaRobots)) {
		return data
	}

	content := string(data)
	content = strings.Replace(content, metaProperty, fmt.Sprintf("%s\n  %s", metaProperty, metaRobots), 1)

	return []byte(content)
}

func (s *Site) getTemplate(templatesFS http.FileSystem, name string, extraFuncs template.FuncMap) (*template.Template, error) {
	readFile := func(fs http.FileSystem, path string) ([]byte, error) {
		f, err := fs.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	tmpl := template.New(rootTemplateName)
	tmpl.Funcs(template.FuncMap{
		"asset": func(path string) string {
			return s.AssetsBase.ResolveReference(&url.URL{Path: path}).String()
		},
		"assetsFromVersion": func(version, path string) string {
			return s.AssetsBase.ResolveReference(&url.URL{Path: path, RawQuery: version}).String()
		},
		"contentFileExists": func(version, path string) bool {
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
		if name == rootTemplateName && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("read template %s", path))
		}
		if name == documentTemplateName {
			// We need to patch the template, since if we're loading an old version the template won't have the
			// nofollow, noindex seo tag
			data = patchTemplateForSEO(data)
		}
		if _, err := tmpl.Parse(string(data)); err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("parse template %s", path))
		}
	}
	return tmpl, nil
}

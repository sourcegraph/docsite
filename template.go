package docsite

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
)

const (
	rootTemplateName     = "root"
	documentTemplateName = "document"
	searchTemplateName   = "search"
)

func (s *Site) getTemplate(templatesFS http.FileSystem, name string, extraFuncs template.FuncMap) (*template.Template, error) {
	tmpl := template.New(rootTemplateName)
	tmpl.Funcs(template.FuncMap{
		"asset": func(path string) string {
			return s.AssetsBase.ResolveReference(&url.URL{Path: path}).String()
		},
		"subtract":   func(a, b int) int { return a - b },
		"replace":    strings.Replace,
		"trimPrefix": strings.TrimPrefix,
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
		if _, err := tmpl.Parse(string(data)); err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("parse template %s", path))
		}
	}
	return tmpl, nil
}

package docsite

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/pkg/errors"
)

func parseTemplates(templatesFS http.FileSystem, funcs template.FuncMap) (*template.Template, error) {
	tmpl := template.New("root")
	tmpl.Funcs(funcs)

	// Read all template files (recursively).
	err := WalkFileSystem(templatesFS, func(path string) error {
		if filepath.Ext(path) != ".html" {
			return nil // not a template
		}
		data, err := ReadFile(templatesFS, path)
		if err != nil {
			return errors.WithMessage(err, fmt.Sprintf("read template %s", path))
		}
		if _, err := tmpl.Parse(string(data)); err != nil {
			return errors.WithMessage(err, fmt.Sprintf("parse template %s", path))
		}
		return nil
	})
	return tmpl, errors.WithMessage(err, "walking templates")
}

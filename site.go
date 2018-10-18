package docsite

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	pathpkg "path"
	"regexp"

	"github.com/pkg/errors"
	"github.com/sourcegraph/docsite/markdown"
)

// Site represents a documentation site, including all of its templates, assets, and content.
type Site struct {
	// Templates is the file system containing the Go html/template templates used to render site
	// pages
	Templates http.FileSystem

	// Content is the file system containing the Markdown files and assets (e.g., images) embedded
	// in them.
	Content http.FileSystem

	// Base is the base URL (typically including only the path, such as "/" or "/help/") where the
	// site is available.
	Base *url.URL

	// Assets is the file system containing the site-wide static asset files (e.g., global styles
	// and logo).
	Assets http.FileSystem

	// AssetsBase is the base URL (sometimes only including the path, such as "/assets/") where the
	// assets are available.
	AssetsBase *url.URL

	// CheckIgnoreURLPattern is a regexp matching URLs to ignore in the Check method.
	CheckIgnoreURLPattern *regexp.Regexp
}

// newContentPage creates a new ContentPage in the site.
func (s *Site) newContentPage(filePath string, data []byte) (*ContentPage, error) {
	path := contentFilePathToPath(filePath)
	doc, err := markdown.Run(data, markdown.Options{
		Base:                      s.Base.ResolveReference(&url.URL{Path: pathpkg.Dir(filePath) + "/"}),
		ContentFilePathToLinkPath: contentFilePathToPath,
	})
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("run Markdown for %s", filePath))
	}
	return &ContentPage{
		Path:        path,
		FilePath:    filePath,
		Data:        data,
		Doc:         *doc,
		Breadcrumbs: makeBreadcrumbEntries(path),
	}, nil
}

// AllContentPages returns a list of all content pages in the site.
func (s *Site) AllContentPages() ([]*ContentPage, error) {
	var pages []*ContentPage
	err := WalkFileSystem(s.Content, func(path string) error {
		if isContentPage(path) {
			page, err := s.ReadContentPage(path)
			if err != nil {
				return err
			}
			pages = append(pages, page)
		}
		return nil
	})
	return pages, err
}

// ReadContentPage reads the content page at the given file path on disk.
func (s *Site) ReadContentPage(filePath string) (*ContentPage, error) {
	data, err := ReadFile(s.Content, filePath)
	if err != nil {
		return nil, err
	}
	return s.newContentPage(filePath, data)
}

// ResolveContentPage looks up the content page at the given path (which generally comes from a
// URL). The path may omit the ".md" file extension and the "/index" or "/index.md" suffix.
//
// If the resulting ContentPage differs from the path argument, the caller should (if possible)
// communicate a redirect.
func (s *Site) ResolveContentPage(path string) (*ContentPage, error) {
	filePath, data, err := resolveAndReadAll(s.Content, path)
	if err != nil {
		return nil, err
	}
	return s.newContentPage(filePath, data)
}

// RenderContentPage renders a content page using the template.
func (s *Site) RenderContentPage(page *ContentPage) ([]byte, error) {
	funcs := template.FuncMap{
		"asset": func(path string) string {
			return s.AssetsBase.ResolveReference(&url.URL{Path: path}).String()
		},
		"markdown": func(page ContentPage) template.HTML {
			return template.HTML(page.Doc.HTML)
		},
	}
	tmpl, err := parseTemplates(s.Templates, funcs)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, page); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

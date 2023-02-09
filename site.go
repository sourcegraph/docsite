package docsite

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"github.com/sourcegraph/docsite/markdown"
)

// VersionedFileSystem represents multiple versions of an http.FileSystem.
type VersionedFileSystem interface {
	OpenVersion(ctx context.Context, version string) (http.FileSystem, error)
}

// Site represents a documentation site, including all of its templates, assets, and content.
type Site struct {
	// Content is the versioned file system containing the Markdown files and assets (e.g., images)
	// embedded in them.
	Content VersionedFileSystem

	// ContentExcludePattern is a regexp matching file paths to exclude in the content file system.
	ContentExcludePattern *regexp.Regexp

	// Base is the base URL (typically including only the path, such as "/" or "/help/") where the
	// site is available.
	Base *url.URL

	// Root is the root URL that is only used for specific cases where an absolute URL is mandatory,
	// such as for opengraph tags in the headers for example. It must include both the scheme and
	// host.
	Root *url.URL

	// Templates is the file system containing the Go html/template templates used to render site
	// pages
	Templates http.FileSystem

	// Assets is the file system containing the site-wide static asset files (e.g., global styles
	// and logo).
	Assets http.FileSystem

	// AssetsBase is the base URL (sometimes only including the path, such as "/assets/") where the
	// assets are available.
	AssetsBase *url.URL

	// Redirects contains a mapping from URL path to redirect destination URL.
	Redirects map[string]*url.URL

	// CheckIgnoreURLPattern is a regexp matching URLs to ignore in the Check method.
	CheckIgnoreURLPattern *regexp.Regexp

	// SkipIndexURLPattern is a regexp matching URLs to ignore when searching. Any files that have a URL that match this
	// pattern will be ignored from the search index.
	SkipIndexURLPattern *regexp.Regexp
}

// newContentPage creates a new ContentPage in the site.
func (s *Site) newContentPage(filePath string, data []byte, contentVersion string) (*ContentPage, error) {
	path := contentFilePathToPath(filePath)
	doc, err := markdown.Run(data, s.markdownOptions(filePath, contentVersion))
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

func (s *Site) markdownOptions(filePath, contentVersion string) markdown.Options {
	var urlPathPrefix string
	if contentVersion != "" {
		urlPathPrefix = "/@" + contentVersion + "/"
	}
	urlPathPrefix = pathpkg.Join(urlPathPrefix, strings.TrimPrefix(pathpkg.Dir(filePath)+"/", "/"))
	if urlPathPrefix != "" {
		urlPathPrefix += "/"
	}

	base := s.Base
	if base == nil {
		base = &url.URL{Path: "/"}
	}

	return markdown.Options{
		Base:                      base.ResolveReference(&url.URL{Path: urlPathPrefix}),
		ContentFilePathToLinkPath: contentFilePathToPath,
		Funcs:                     createMarkdownFuncs(s),
		FuncInfo:                  markdown.FuncInfo{Version: contentVersion},
	}
}

// AllContentPages returns a list of all content pages in the site.
func (s *Site) AllContentPages(ctx context.Context, contentVersion string) ([]*ContentPage, error) {
	content, err := s.Content.OpenVersion(ctx, contentVersion)
	if err != nil {
		return nil, err
	}

	filter := func(path string) bool {
		if !isContentPage(path) {
			return false
		}
		return s.checkIsValidPath(path) == nil
	}

	var pages []*ContentPage
	err = WalkFileSystem(content, filter, func(path string) error {
		data, err := ReadFile(content, path)
		if err != nil {
			return err
		}
		page, err := s.newContentPage(path, data, contentVersion)
		if err != nil {
			return err
		}
		pages = append(pages, page)
		return nil
	})
	return pages, err
}

// ResolveContentPage looks up the content page at the given version and path (which generally comes
// from a URL). The path may omit the ".md" file extension and the "/index" or "/index.md" suffix.
//
// If the resulting ContentPage differs from the path argument, the caller should (if possible)
// communicate a redirect.
func (s *Site) ResolveContentPage(ctx context.Context, contentVersion, path string) (*ContentPage, error) {
	if err := s.checkIsValidPath(path); err != nil {
		return nil, err
	}
	content, err := s.Content.OpenVersion(ctx, contentVersion)
	if err != nil {
		return nil, err
	}
	filePath, data, err := resolveAndReadAll(content, path)
	if err != nil {
		return nil, err
	}
	return s.newContentPage(filePath, data, contentVersion)
}

func (s *Site) checkIsValidPath(path string) error {
	if s.ContentExcludePattern == nil || !s.ContentExcludePattern.MatchString(path) {
		return nil
	}
	return &os.PathError{Op: "open", Path: path, Err: errors.New("path is excluded")}
}

// PageData is the data available to the HTML template used to render a page.
type PageData struct {
	ContentVersion  string // content version string requested
	ContentPagePath string // content page path requested

	ContentVersionNotFoundError bool // whether the requested version was not found
	ContentPageNotFoundError    bool // whether the requested content page was not found

	// Content is the content page, when it is found.
	Content *ContentPage
}

// RenderContentPage renders a content page using the template.
func (s *Site) RenderContentPage(page *PageData) ([]byte, error) {
	tmpl, err := s.getTemplate(s.Templates, documentTemplateName, template.FuncMap{
		"markdown": func(page ContentPage) template.HTML {
			return template.HTML(page.Doc.HTML)
		},
	})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, page); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

package docsite

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// versionPattern matches version strings like @5.2, @5.2.0, etc. and captures major and minor version numbers
var versionPattern = regexp.MustCompile(`^@(\d+)\.(\d+)(?:\.(\d+))?$`)

// shouldRedirectVersion returns true for versions ≥ 5.2 (format: @major.minor[.patch])
func shouldRedirectVersion(version string) bool {
	matches := versionPattern.FindStringSubmatch(version)
	if len(matches) < 3 {
		return false
	}
	
	major, err1 := strconv.Atoi(matches[1])
	minor, err2 := strconv.Atoi(matches[2])
	if err1 != nil || err2 != nil {
		return false
	}
	
	return major > 5 || (major == 5 && minor >= 2)
}

// Handler returns an http.Handler that serves the site.
func (s *Site) Handler() http.Handler {
	m := http.NewServeMux()

	const (
		cacheMaxAge0     = "max-age=0"
		cacheMaxAgeShort = "max-age=60"
		cacheMaxAgeLong  = "max-age=300"
	)
	isNoCacheRequest := func(r *http.Request) bool {
		return r.Header.Get("Cache-Control") == "no-cache"
	}
	isRedirect := func(path string) *url.URL {
		requestPathWithLeadingSlash := path
		if !strings.HasPrefix(requestPathWithLeadingSlash, "/") {
			requestPathWithLeadingSlash = "/" + requestPathWithLeadingSlash
		}
		if redirectTo, ok := s.Redirects[requestPathWithLeadingSlash]; ok {
			return redirectTo
		}
		return nil
	}
	setCacheControl := func(w http.ResponseWriter, r *http.Request, cacheControl string) {
		if isNoCacheRequest(r) {
			w.Header().Set("Cache-Control", cacheMaxAge0)
		} else {
			w.Header().Set("Cache-Control", cacheControl)
		}
	}

	// Serve assets using http.FileServer.
	if s.AssetsBase != nil {
		assets, err := s.GetResources("assets", "")
		if err != nil {
			panic("failed to open assets: " + err.Error())
		}

		assetsFileServer := http.FileServer(assets)
		m.Handle(s.AssetsBase.Path, http.StripPrefix(s.AssetsBase.Path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery != "" {
				versionAssets, err := s.GetResources("assets", r.URL.RawQuery)
				if err != nil {
					http.Error(w, "version assets error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				assetsFileServer = http.FileServer(versionAssets)
			}
			setCacheControl(w, r, cacheMaxAgeLong)
			assetsFileServer.ServeHTTP(w, r)
		})))
	}

	var basePath string
	if s.Base != nil {
		basePath = s.Base.Path
	} else {
		basePath = "/"
	}

	// Serve search.
	m.Handle(path.Join(basePath, "search"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		queryStr := r.URL.Query().Get("q")
		contentVersion := r.URL.Query().Get("v")
		result, err := s.Search(r.Context(), contentVersion, queryStr)
		if err != nil {
			w.Header().Set("Cache-Control", cacheMaxAge0)
			http.Error(w, "search error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var respData []byte
		if r.Method == "GET" {
			respData, err = s.renderSearchPage(contentVersion, queryStr, result)
			if err != nil {
				w.Header().Set("Cache-Control", cacheMaxAge0)
				http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		setCacheControl(w, r, cacheMaxAgeShort)
		if r.Method == "GET" {
			_, _ = w.Write(respData)
		}
	}))

	// Serve content.
	m.Handle(basePath, http.StripPrefix(basePath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if redirectTo := isRedirect(r.URL.Path); redirectTo != nil {
			http.Redirect(w, r, redirectTo.String(), http.StatusPermanentRedirect)
			return
		}

		// Support requests for other versions of content.
		var contentVersion string
		if strings.HasPrefix(r.URL.Path, "@") {
			end := strings.Index(r.URL.Path[1:], "/")
			var urlPath string
			if end == -1 {
				urlPath = ""
				contentVersion = r.URL.Path[1:]
			} else {
				urlPath = r.URL.Path[1+end+1:]
				contentVersion = r.URL.Path[1 : 1+end]
			}
			
			// Redirect versions ≥ 5.2 to new docs domain with path preservation
			version := "@" + contentVersion
			if shouldRedirectVersion(version) {
				newURL := "https://www.sourcegraph.com/docs/@" + contentVersion
				if urlPath != "" {
					newURL += "/" + urlPath
				}
				http.Redirect(w, r, newURL, http.StatusPermanentRedirect)
				return
			}
			
			r = requestShallowCopyWithURLPath(r, urlPath)
		}

		if IsContentAsset(r.URL.Path) {
			// Serve non-Markdown content files (such as images) using http.FileServer.
			content, err := s.Content.OpenVersion(r.Context(), contentVersion)
			if err != nil {
				w.Header().Set("Cache-Control", cacheMaxAge0)

				if os.IsNotExist(err) {
					http.Error(w, "content version not found", http.StatusNotFound)
				} else {
					http.Error(w, "content version error: "+err.Error(), http.StatusInternalServerError)
				}
				return
			}
			setCacheControl(w, r, cacheMaxAgeLong)
			http.FileServer(content).ServeHTTP(w, r)
			return
		}

		data := PageData{
			ContentVersion:  contentVersion,
			ContentPagePath: r.URL.Path,
		}
		content, err := s.Content.OpenVersion(r.Context(), contentVersion)
		if err != nil {
			// Version not found.
			if !os.IsNotExist(err) {
				w.Header().Set("Cache-Control", cacheMaxAge0)
				http.Error(w, "content version error: "+err.Error(), http.StatusNotFound)
				return
			}
			data.ContentVersionNotFoundError = true
		} else {
			// Version found.
			filePath, fileData, err := resolveAndReadAll(content, r.URL.Path)
			if err == nil {
				// Strip trailing slashes for consistency.
				if strings.HasSuffix(r.URL.Path, "/") {
					http.Redirect(w, r, path.Join(basePath, strings.TrimSuffix(r.URL.Path, "/")), http.StatusMovedPermanently)
					return
				}

				// Content page found.
				data.Content, err = s.newContentPage(filePath, fileData, contentVersion)
			}
			if err != nil {
				// Content page not found.
				if !os.IsNotExist(err) {
					w.Header().Set("Cache-Control", cacheMaxAge0)
					http.Error(w, "content error: "+err.Error(), http.StatusInternalServerError)
					return
				}

				// If this is a versioned request, let's see if we have a
				// redirect that would have matched an unversioned request. We
				// can't really make this worse, after all, and we now have the
				// version cached.
				if contentVersion != "" {
					if to := isRedirect(r.URL.Path); to != nil {
						// We need to ensure we redirect to a page on the same
						// version, and this needs to be an absolute path, so we
						// prepend a slash.
						http.Redirect(w, r, "/"+filepath.Join("@"+contentVersion, to.String()), http.StatusPermanentRedirect)
						return
					}
				}

				data.ContentPageNotFoundError = true
			}
		}

		var respData []byte
		if r.Method == "GET" {
			var err error
			respData, err = s.RenderContentPage(&data)
			if err != nil {
				w.Header().Set("Cache-Control", cacheMaxAge0)
				http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Don't cache errors; do cache on success.
		if data.Content == nil {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Cache-Control", cacheMaxAge0)
		} else {
			setCacheControl(w, r, cacheMaxAgeShort)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == "GET" {
			_, _ = w.Write(respData)
		}
	})))

	return m
}

func requestShallowCopyWithURLPath(r *http.Request, path string) *http.Request {
	r2 := new(http.Request)
	*r2 = *r
	r2.URL = new(url.URL)
	*r2.URL = *r.URL
	r2.URL.Path = path
	return r2
}

// IsContentAsset reports whether the file in the site contents file system is a content asset
// (i.e., not a Markdown file). It typically matches .png, .gif, and .svg files.
func IsContentAsset(urlPath string) bool {
	return filepath.Ext(urlPath) != "" && !isContentPage(urlPath)
}

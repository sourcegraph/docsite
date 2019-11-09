package docsite

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Handler returns an http.Handler that serves the site.
func (s *Site) Handler() http.Handler {
	m := http.NewServeMux()

	const (
		cacheMaxAge0     = "max-age=0"
		cacheMaxAgeShort = "max-age=60"
		cacheMaxAgeLong  = "max-age=3600"
	)
	isNoCacheRequest := func(r *http.Request) bool {
		return r.Header.Get("Cache-Control") == "no-cache"
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
		assetsFileServer := http.FileServer(s.Assets)
		m.Handle(s.AssetsBase.Path, http.StripPrefix(s.AssetsBase.Path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setCacheControl(w, r, cacheMaxAgeLong)
			assetsFileServer.ServeHTTP(w, r)
		})))
	}

	// Serve content.
	var basePath string
	if s.Base != nil {
		basePath = s.Base.Path
	} else {
		basePath = "/"
	}
	m.Handle(basePath, http.StripPrefix(basePath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		{
			requestPathWithLeadingSlash := r.URL.Path
			if !strings.HasPrefix(requestPathWithLeadingSlash, "/") {
				requestPathWithLeadingSlash = "/" + requestPathWithLeadingSlash
			}
			if redirectTo, ok := s.Redirects[requestPathWithLeadingSlash]; ok {
				http.Redirect(w, r, redirectTo.String(), http.StatusPermanentRedirect)
				return
			}
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
				http.Error(w, "content version error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			data.ContentVersionNotFoundError = true
		} else {
			// Version found.
			filePath, fileData, err := resolveAndReadAll(content, r.URL.Path)
			if err == nil {
				// Content page found.
				data.Content, err = s.newContentPage(r.Context(), filePath, fileData, contentVersion)
			}
			if err != nil {
				// Content page not found.
				if !os.IsNotExist(err) {
					w.Header().Set("Cache-Control", cacheMaxAge0)
					http.Error(w, "content error: "+err.Error(), http.StatusInternalServerError)
					return
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
			w.Write(respData)
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

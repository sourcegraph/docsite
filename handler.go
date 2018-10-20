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

	const longCache = "max-age=3600"

	// Serve assets using http.FileServer.
	if s.AssetsBase != nil {
		assetsFileServer := http.FileServer(s.Assets)
		m.Handle(s.AssetsBase.Path, http.StripPrefix(s.AssetsBase.Path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", longCache)
			assetsFileServer.ServeHTTP(w, r)
		})))
	}

	// Serve content.
	m.Handle(s.Base.Path, http.StripPrefix(s.Base.Path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
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
			r = requestShallowCopyWithURLPath(r, urlPath)
		}

		if IsContentAsset(r.URL.Path) {
			// Serve non-Markdown content files (such as images) using http.FileServer.
			content, err := s.Content.OpenVersion(r.Context(), contentVersion)
			if err != nil {
				w.Header().Set("Cache-Control", "max-age=0")
				if os.IsNotExist(err) {
					http.Error(w, "content version not found", http.StatusNotFound)
				} else {
					http.Error(w, "content version error: "+err.Error(), http.StatusInternalServerError)
				}
				return
			}
			w.Header().Set("Cache-Control", longCache)
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
				w.Header().Set("Cache-Control", "max-age=0")
				http.Error(w, "content version error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			data.ContentVersionNotFoundError = true
		} else {
			// Version found.
			filePath, fileData, err := resolveAndReadAll(content, r.URL.Path)
			if err == nil {
				// Content page found.
				data.Content, err = s.newContentPage(filePath, fileData, contentVersion)
			}
			if err != nil {
				// Content page not found.
				if !os.IsNotExist(err) {
					w.Header().Set("Cache-Control", "max-age=0")
					http.Error(w, "content error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				data.ContentPageNotFoundError = true
			}
		}

		respData, err := s.RenderContentPage(&data)
		if err != nil {
			w.Header().Set("Cache-Control", "max-age=0")
			http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Don't cache errors; do cache on success.
		if data.Content == nil {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Cache-Control", "max-age=0")
		} else {
			w.Header().Set("Cache-Control", "max-age=60")
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

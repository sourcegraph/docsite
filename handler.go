package docsite

import (
	"net/http"
	"os"
	"path/filepath"
)

// Handler returns an http.Handler that serves the site.
func (s *Site) Handler() http.Handler {
	m := http.NewServeMux()

	// Serve assets using http.FileServer.
	if s.AssetsBase != nil {
		m.Handle(s.AssetsBase.Path, http.StripPrefix(s.AssetsBase.Path, http.FileServer(s.Assets)))
	}

	// Serve non-Markdown content files using http.FileServer.
	contentAssetsHandler := http.FileServer(s.Content)

	// Serve content.
	m.Handle(s.Base.Path, http.StripPrefix(s.Base.Path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsContentAsset(r.URL.Path) {
			// Use contentAssetsHandler for non-Markdown content (such as images).
			contentAssetsHandler.ServeHTTP(w, r)
			return
		}

		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		page, err := s.ResolveContentPage(r.URL.Path)
		if err != nil {
			w.Header().Set("Cache-Control", "max-age=0")
			if os.IsNotExist(err) {
				http.Error(w, "not found", http.StatusNotFound)
			} else {
				http.Error(w, "error: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		data, err := s.RenderContentPage(page)
		if err != nil {
			w.Header().Set("Cache-Control", "max-age=0")
			http.Error(w, "error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "max-age=0")
		if r.Method == "GET" {
			w.Write(data)
		}
	})))

	return m
}

// IsContentAsset reports whether the file in the site contents file system is a content asset
// (i.e., not a Markdown file). It typically matches .png, .gif, and .svg files.
func IsContentAsset(urlPath string) bool {
	return filepath.Ext(urlPath) != "" && !isContentPage(urlPath)
}

package docsite

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

// gifData is GIF image data for a 1x1 transparent pixel.
var gifData, _ = base64.RawStdEncoding.DecodeString("R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw")

func TestSite_Handler(t *testing.T) {
	checkResponseHTTPOK := func(t *testing.T, rr *httptest.ResponseRecorder) {
		t.Helper()
		if want := http.StatusOK; rr.Code != want {
			t.Errorf("got HTTP status %d, want %d", rr.Code, want)
		}
	}
	checkContentPageResponse := func(t *testing.T, rr *httptest.ResponseRecorder) {
		t.Helper()
		if got, want := rr.Header().Get("Content-Type"), "text/html; charset=utf-8"; got != want {
			t.Errorf("got Content-Type %q, want %q", got, want)
		}
	}

	site := Site{
		Templates: httpfs.New(mapfs.New(map[string]string{
			"template.html": `
{{define "root" -}}
{{range .Breadcrumbs}}{{.Label}} ({{.URL}}){{if not .IsActive}} / {{end}}{{end}}
{{markdown .}}
{{- end}}`,
		})),
		Content: httpfs.New(mapfs.New(map[string]string{
			"index.md":      "z [a/b](a/b/index.md)",
			"a/b/index.md":  "e",
			"a/b/c.md":      "d",
			"a/b/img/f.gif": string(gifData),
		})),
		Base: &url.URL{Path: "/"},
		Assets: httpfs.New(mapfs.New(map[string]string{
			"g.gif": string(gifData),
		})),
		AssetsBase: &url.URL{Path: "/assets/"},
	}
	handler := site.Handler()

	t.Run("content", func(t *testing.T) {
		t.Run("root", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/", nil)
			handler.ServeHTTP(rr, req)
			checkResponseHTTPOK(t, rr)
			checkContentPageResponse(t, rr)
		})

		t.Run("index", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/a/b", nil)
			handler.ServeHTTP(rr, req)
			checkResponseHTTPOK(t, rr)
			checkContentPageResponse(t, rr)
		})

		t.Run("page", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/a/b/c", nil)
			handler.ServeHTTP(rr, req)
			checkResponseHTTPOK(t, rr)
			checkContentPageResponse(t, rr)
		})

		t.Run("asset", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/a/b/img/f.gif", nil)
			handler.ServeHTTP(rr, req)
			checkResponseHTTPOK(t, rr)
			if got, want := rr.Header().Get("Content-Type"), "image/gif"; got != want {
				t.Errorf("got Content-Type %q, want %q", got, want)
			}
		})
	})

	t.Run("asset", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/assets/g.gif", nil)
		handler.ServeHTTP(rr, req)
		checkResponseHTTPOK(t, rr)
		if got, want := rr.Header().Get("Content-Type"), "image/gif"; got != want {
			t.Errorf("got Content-Type %q, want %q", got, want)
		}
	})
}

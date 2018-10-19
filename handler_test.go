package docsite

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
		Content: versionedFileSystem{
			"": httpfs.New(mapfs.New(map[string]string{
				"index.md":      "z [a/b](a/b/index.md)",
				"a/b/index.md":  "e",
				"a/b/c.md":      "d",
				"a/b/img/f.gif": string(gifData),
			})),
			"otherversion": httpfs.New(mapfs.New(map[string]string{
				"index.md": "other version index",
				"a.md":     "other version a",
			})),
		},
		Base: &url.URL{Path: "/"},
		Assets: httpfs.New(mapfs.New(map[string]string{
			"g.gif": string(gifData),
		})),
		AssetsBase: &url.URL{Path: "/assets/"},
	}
	handler := site.Handler()

	t.Run("content", func(t *testing.T) {
		t.Run("default version", func(t *testing.T) {
			t.Run("root", func(t *testing.T) {
				rr := httptest.NewRecorder()
				rr.Body = new(bytes.Buffer)
				req, _ := http.NewRequest("GET", "/", nil)
				handler.ServeHTTP(rr, req)
				checkResponseHTTPOK(t, rr)
				checkContentPageResponse(t, rr)
				if want := `z <a href="/a/b">a/b</a>`; !strings.Contains(rr.Body.String(), want) {
					t.Errorf("got body %q, want contains %q", rr.Body.String(), want)
				}
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

		t.Run("other version", func(t *testing.T) {
			t.Run("root", func(t *testing.T) {
				rr := httptest.NewRecorder()
				rr.Body = new(bytes.Buffer)
				req, _ := http.NewRequest("GET", "/@otherversion", nil)
				handler.ServeHTTP(rr, req)
				checkResponseHTTPOK(t, rr)
				checkContentPageResponse(t, rr)
				if want := "other version index"; !strings.Contains(rr.Body.String(), want) {
					t.Errorf("got body %q, want contains %q", rr.Body.String(), want)
				}
			})

			t.Run("page", func(t *testing.T) {
				rr := httptest.NewRecorder()
				rr.Body = new(bytes.Buffer)
				req, _ := http.NewRequest("GET", "/@otherversion/a", nil)
				handler.ServeHTTP(rr, req)
				checkResponseHTTPOK(t, rr)
				checkContentPageResponse(t, rr)
				if want := "other version a"; !strings.Contains(rr.Body.String(), want) {
					t.Errorf("got body %q, want contains %q", rr.Body.String(), want)
				}
			})
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

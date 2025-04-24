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
	checkResponseStatus := func(t *testing.T, rr *httptest.ResponseRecorder, want int) {
		t.Helper()
		if rr.Code != want {
			t.Errorf("got HTTP status %d, want %d", rr.Code, want)
		}
	}
	checkResponseHTTPOK := func(t *testing.T, rr *httptest.ResponseRecorder) {
		t.Helper()
		checkResponseStatus(t, rr, http.StatusOK)
	}
	checkContentPageResponse := func(t *testing.T, rr *httptest.ResponseRecorder) {
		t.Helper()
		if got, want := rr.Header().Get("Content-Type"), "text/html; charset=utf-8"; got != want {
			t.Errorf("got Content-Type %q, want %q", got, want)
		}
	}

	site := Site{
		Content: versionedFileSystem{
			"": httpfs.New(mapfs.New(map[string]string{
				"index.md":                       "z [a/b](a/b/index.md)",
				"a/b/index.md":                   "e",
				"a/b/c.md":                       "d",
				"a/b/img/f.gif":                  string(gifData),
				"_resources/templates/root.html": `{{block "content" .}}empty{{end}}`,
				"_resources/templates/document.html": `
					{{define "content" -}}
						{{with .Content}}
							{{range .Breadcrumbs}}{{.Label}} ({{.URL}}){{if not .IsActive}} / {{end}}{{end}}
							{{markdown .}}
						{{else}}
							{{if .ContentVersionNotFoundError}}content version not found{{end}}
							{{if .ContentPageNotFoundError}}content page not found{{end}}
						{{end}}
				 	{{- end}}`,
				"_resources/templates/search.html": `
					 {{define "content" -}}
					 query "{{.Query}}":
					 {{- range $dr := .Result.DocumentResults -}}
					 	{{range $sr := .SectionResults -}}
					 		<a href="/{{$dr.ID}}#{{$sr.ID}}">{{range $sr.Excerpts}}{{.}}{{end}}</a>
					 	{{end -}}
					 {{end -}}
					 {{- end}}`,
				"_resources/assets/g.gif": string(gifData),
			})),
			"otherversion": httpfs.New(mapfs.New(map[string]string{
				"index.md":                       "other version index",
				"a.md":                           "other version a",
				"_resources/templates/root.html": `{{block "content" .}}empty{{end}}`,
				"_resources/templates/document.html": `
				{{define "content" -}}
					{{with .Content}}
						{{range .Breadcrumbs}}{{.Label}} ({{.URL}}){{if not .IsActive}} / {{end}}{{end}}
						{{markdown .}}
					{{else}}
						{{if .ContentVersionNotFoundError}}content version not found{{end}}
						{{if .ContentPageNotFoundError}}content page not found{{end}}
					{{end}}
				 {{- end}}`,
			})),
		},
		Base:       &url.URL{Path: "/"},
		AssetsBase: &url.URL{Path: "/assets/"},
		Redirects: map[string]*url.URL{
			"/redirect-from": {Path: "/redirect-to"},
		},
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

			t.Run("index page with trailing slash", func(t *testing.T) {
				rr := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/a/b/", nil)
				handler.ServeHTTP(rr, req)
				checkResponseStatus(t, rr, http.StatusMovedPermanently)
				if got, want := rr.Header().Get("Location"), "/a/b"; got != want {
					t.Errorf("got Location %q, want %q", got, want)
				}
			})

			t.Run("non-index page with trailing slash", func(t *testing.T) {
				rr := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/a/b/c/", nil)
				handler.ServeHTTP(rr, req)
				checkResponseStatus(t, rr, http.StatusMovedPermanently)
				if got, want := rr.Header().Get("Location"), "/a/b/c"; got != want {
					t.Errorf("got Location %q, want %q", got, want)
				}
			})

			t.Run("non-existent page with trailing slash", func(t *testing.T) {
				rr := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/a/b/d/", nil)
				handler.ServeHTTP(rr, req)
				checkResponseStatus(t, rr, http.StatusNotFound)
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

		t.Run("version not found", func(t *testing.T) {
			t.Run("root", func(t *testing.T) {
				rr := httptest.NewRecorder()
				rr.Body = new(bytes.Buffer)
				req, _ := http.NewRequest("GET", "/@badversion", nil)
				handler.ServeHTTP(rr, req)
				checkResponseStatus(t, rr, http.StatusNotFound)
				if want := "content version error: version not found\n"; !strings.Contains(rr.Body.String(), want) {
					t.Errorf("got body %q, want contains %q", rr.Body.String(), want)
				}
			})

			t.Run("page", func(t *testing.T) {
				rr := httptest.NewRecorder()
				rr.Body = new(bytes.Buffer)
				req, _ := http.NewRequest("GET", "/@badversion/a", nil)
				handler.ServeHTTP(rr, req)
				checkResponseStatus(t, rr, http.StatusNotFound)
				if want := "content version error: version not found\n"; !strings.Contains(rr.Body.String(), want) {
					t.Errorf("got body %q, want contains %q", rr.Body.String(), want)
				}
			})
		})

		t.Run("page not found", func(t *testing.T) {
			t.Run("default version", func(t *testing.T) {
				rr := httptest.NewRecorder()
				rr.Body = new(bytes.Buffer)
				req, _ := http.NewRequest("GET", "/doesntexist", nil)
				handler.ServeHTTP(rr, req)
				checkResponseStatus(t, rr, http.StatusNotFound)
				checkContentPageResponse(t, rr)
				if want := "content page not found"; !strings.Contains(rr.Body.String(), want) {
					t.Errorf("got body %q, want contains %q", rr.Body.String(), want)
				}
			})

			t.Run("other version", func(t *testing.T) {
				rr := httptest.NewRecorder()
				rr.Body = new(bytes.Buffer)
				req, _ := http.NewRequest("GET", "/@otherversion/doesntexist", nil)
				handler.ServeHTTP(rr, req)
				checkResponseStatus(t, rr, http.StatusNotFound)
				checkContentPageResponse(t, rr)
				if want := "content page not found"; !strings.Contains(rr.Body.String(), want) {
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

	t.Run("redirect", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/redirect-from", nil)
		handler.ServeHTTP(rr, req)
		checkResponseStatus(t, rr, http.StatusPermanentRedirect)
		if got, want := rr.Header().Get("Location"), "/redirect-to"; got != want {
			t.Errorf("got redirect Location %q, want %q", got, want)
		}
	})

	t.Run("search", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rr.Body = new(bytes.Buffer)
		req, _ := http.NewRequest("GET", "/search?q=d", nil)
		handler.ServeHTTP(rr, req)
		checkResponseHTTPOK(t, rr)
		checkContentPageResponse(t, rr)
		if want := `query "d":<a href="/a/b/c.md#">d</a>`; !strings.Contains(rr.Body.String(), want) {
			t.Errorf("got body %q, want contains %q", rr.Body.String(), want)
		}
	})
	
	t.Run("version redirects", func(t *testing.T) {
		t.Run("version 5.1 - no redirect", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/@5.1", nil)
			handler.ServeHTTP(rr, req)
			checkResponseStatus(t, rr, http.StatusNotFound)
		})

		t.Run("version 5.2 - should redirect", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/@5.2", nil)
			handler.ServeHTTP(rr, req)
			checkResponseStatus(t, rr, http.StatusPermanentRedirect)
			if got, want := rr.Header().Get("Location"), "https://www.sourcegraph.com/docs/5.2/"; got != want {
				t.Errorf("got redirect Location %q, want %q", got, want)
			}
		})

		t.Run("version 5.3 - should redirect", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/@5.3", nil)
			handler.ServeHTTP(rr, req)
			checkResponseStatus(t, rr, http.StatusPermanentRedirect)
			if got, want := rr.Header().Get("Location"), "https://www.sourcegraph.com/docs/5.3/"; got != want {
				t.Errorf("got redirect Location %q, want %q", got, want)
			}
		})

		t.Run("version with path - should redirect with path", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/@5.3/some/path", nil)
			handler.ServeHTTP(rr, req)
			checkResponseStatus(t, rr, http.StatusPermanentRedirect)
			if got, want := rr.Header().Get("Location"), "https://www.sourcegraph.com/docs/5.3/some/path"; got != want {
				t.Errorf("got redirect Location %q, want %q", got, want)
			}
		})

		t.Run("major version > 5 - should redirect", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/@6.0", nil)
			handler.ServeHTTP(rr, req)
			checkResponseStatus(t, rr, http.StatusPermanentRedirect)
			if got, want := rr.Header().Get("Location"), "https://www.sourcegraph.com/docs/6.0/"; got != want {
				t.Errorf("got redirect Location %q, want %q", got, want)
			}
		})

		t.Run("version with patch - should redirect", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/@5.3.1", nil)
			handler.ServeHTTP(rr, req)
			checkResponseStatus(t, rr, http.StatusPermanentRedirect)
			if got, want := rr.Header().Get("Location"), "https://www.sourcegraph.com/docs/5.3.1/"; got != want {
				t.Errorf("got redirect Location %q, want %q", got, want)
			}
		})
	})
}

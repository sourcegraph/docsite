package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/sourcegraph/docsite"
)

var (
	srcDir    = flag.String("sources", "../sourcegraph/doc", "path to dir containing .md source files")
	tmplFile  = flag.String("templates", "templates", "path to dir containing .html template files")
	assetsDir = flag.String("assets", "assets", "path to dir containing assets (styles, scripts, images, etc.)")
	outDir    = flag.String("out", "out", "path to output dir where .html files are written")
	httpAddr  = flag.String("http", ":8000", "HTTP listen address for previewing")
)

const (
	assetsURLPathPrefix = "/assets/"
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	gen := docsite.Generator{
		Sources:             http.Dir(*srcDir),
		Templates:           http.Dir(*tmplFile),
		AssetsURLPathPrefix: assetsURLPathPrefix,
	}
	log.Println("# Preview HTTP server listening on", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, &handler{gen: gen}))
}

type handler struct {
	gen docsite.Generator
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.URL.Path, assetsURLPathPrefix) {
		http.StripPrefix(assetsURLPathPrefix, http.FileServer(http.Dir(*assetsDir))).ServeHTTP(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	out, err := h.gen.Generate(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
		} else {
			http.Error(w, "error: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=0")
	w.Write(out)
}

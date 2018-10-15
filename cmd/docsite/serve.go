package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/sourcegraph/docsite"
)

func init() {
	flagSet := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		httpAddr = flagSet.String("http", ":8000", "HTTP listen address for previewing")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)
		log.Println("# Preview HTTP server listening on", *httpAddr)
		return http.ListenAndServe(*httpAddr, newHandler())
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "serve a live preview of the site",
		LongDescription:  "The serve subcommand serves a live preview of the site over HTTP. After changing a source (Markdown) or template file, changes are immediately visible after reloading the page.",
		handler:          handler,
	})
}

type handler struct {
	gen docsite.Generator
}

func newHandler() *handler {
	return &handler{gen: generatorFromFlags()}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.URL.Path, assetsURLPathPrefix) {
		http.StripPrefix(assetsURLPathPrefix, http.FileServer(http.Dir(*assetsDir))).ServeHTTP(w, r)
		return
	}

	// Serve non-Markdown file (e.g., image) in sources.
	f, err := h.gen.Sources.Open(r.URL.Path)
	if err == nil {
		fi, err := f.Stat()
		if err == nil && fi.Mode().IsRegular() {
			w.Header().Set("Cache-Control", "max-age=0")
			_, _ = io.Copy(w, f)
			return
		}
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	out, err := h.gen.Generate(path, false)
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

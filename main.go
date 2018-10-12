package main

import (
	"bytes"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	gfm "github.com/shurcooL/github_flavored_markdown"
)

var (
	srcDir    = flag.String("src", "../sourcegraph/doc", "path to dir containing .md source files")
	tmplFile  = flag.String("tmpl", "template.html", "path to .html template file")
	assetsDir = flag.String("assets", "assets", "path to dir containing assets (styles, scripts, images, etc.)")
	outDir    = flag.String("out", "out", "path to output dir where .html files are written")
	httpAddr  = flag.String("http", ":8000", "HTTP listen address for previewing")
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	gen := generator{src: sourceDir(*srcDir), tmplFile: *tmplFile}
	log.Println("# Preview HTTP server listening on", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, &handler{gen: gen}))
}

const (
	indexName           = "index"
	assetsURLPathPrefix = "/assets/"
)

type sourceDir string

// resolveAndReadAll resolves a URL path to a file path, adding a file extension (.md) and a
// directory index filename as needed. It also returns the file content.
func (dir sourceDir) resolveAndReadAll(path string) (filePath string, data []byte, err error) {
	if path == "" {
		// Special-case: the top-level index file is README.md not index.md.
		path = "README"
	}

	filePath = path + ".md"
	data, err = ioutil.ReadFile(filepath.Join(string(dir), filePath))
	if os.IsNotExist(err) && !strings.HasSuffix(path, string(os.PathSeparator)+indexName) {
		// Try looking up the path as a directory and reading its index file (index.md).
		return dir.resolveAndReadAll(filepath.Join(path, indexName))
	}
	return filePath, data, err
}

type sourceFile struct {
	FilePath    string
	Data        []byte
	Breadcrumbs []breadcrumbEntry
}

type breadcrumbEntry struct {
	Label    string
	URL      string
	IsActive bool
}

func makeBreadcrumbEntries(path string) []breadcrumbEntry {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	entries := make([]breadcrumbEntry, len(parts)+1)
	entries[0] = breadcrumbEntry{
		Label: "Documentation",
		URL:   "/",
	}
	for i, part := range parts {
		entries[i+1] = breadcrumbEntry{
			Label:    part,
			URL:      "/" + pathpkg.Join(parts[:i+1]...),
			IsActive: i == len(parts)-1,
		}
	}
	return entries
}

type generator struct {
	src      sourceDir
	tmplFile string
}

func (g *generator) getTemplate() (*template.Template, error) {
	tmpl := template.New("root")
	tmpl.Funcs(template.FuncMap{
		"asset": func(path string) string {
			return assetsURLPathPrefix + path
		},
		"markdown": func(data []byte) template.HTML {
			return template.HTML(gfm.Markdown(data))
		},
	})
	return tmpl.ParseFiles(g.tmplFile)
}

func (g *generator) getSourceFile(path string) (*sourceFile, error) {
	filePath, data, err := g.src.resolveAndReadAll(path)
	if err != nil {
		return nil, err
	}
	return &sourceFile{
		FilePath:    filePath,
		Data:        data,
		Breadcrumbs: makeBreadcrumbEntries(path),
	}, nil
}

func (g *generator) generate(path string) ([]byte, error) {
	tmpl, err := g.getTemplate()
	if err != nil {
		return nil, err
	}

	src, err := g.getSourceFile(path)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, src); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type handler struct {
	gen generator
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
	out, err := h.gen.generate(path)
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

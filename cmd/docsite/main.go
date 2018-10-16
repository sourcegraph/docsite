package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"text/template"

	"github.com/sourcegraph/docsite"
)

var usage = template.Must(template.New("").Parse(`docsite is a tool for generating static documentation sites from Markdown files and HTML templates.
For more information, see https://github.com/sourcegraph/docsite.

Usage:

  docsite [options] command [command options]

The options are:

{{call .FlagUsage }}
The commands are:
{{range .Commands}}
  {{printf "%- 15s" .NameAndAliases}} {{.ShortDescription}}
{{- end}}

Use "docsite [command] -h" for more information about a command.

`))

var (
	contentDir   = flag.String("content", "../sourcegraph/doc", "path to `dir` containing content (.md files and related images, etc.)")
	baseURLPath  = flag.String("base-url-path", "/", "base `URL path` where doc site lives (examples: /, /help/)")
	templatesDir = flag.String("templates", "templates", "path to `dir` containing .html template files (Go html/template)")
	assetsDir    = flag.String("assets", "assets", "path to `dir` containing site-wide assets (styles, scripts, images, etc.)")
)

// commands contains all registered subcommands.
var commands commander

func main() {
	log.SetFlags(0)
	log.SetPrefix("")
	commands.run(flag.CommandLine, "docsite", usage, os.Args[1:])
}

const (
	assetsURLPathComponent = "assets"
	assetsURLPathPrefix    = "/" + assetsURLPathComponent + "/"
)

func siteFromFlags() docsite.Site {
	return docsite.Site{
		Templates:  http.Dir(*templatesDir),
		Content:    http.Dir(*contentDir),
		Base:       &url.URL{Path: *baseURLPath},
		Assets:     http.Dir(*assetsDir),
		AssetsBase: &url.URL{Path: assetsURLPathPrefix},
	}
}

package main

import (
	"flag"
	"log"
	"net/http"
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
	sourcesDir   = flag.String("sources", "../sourcegraph/doc", "path to `dir` containing .md source files")
	templatesDir = flag.String("templates", "templates", "path to `dir` containing .html template files")
	assetsDir    = flag.String("assets", "assets", "path to `dir` containing assets (styles, scripts, images, etc.)")
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

func generatorFromFlags() docsite.Generator {
	return docsite.Generator{
		Sources:             http.Dir(*sourcesDir),
		Templates:           http.Dir(*templatesDir),
		AssetsURLPathPrefix: assetsURLPathPrefix,
	}
}

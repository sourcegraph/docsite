package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"
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
	// commandLine is the global flag set. It is used instead of flag.CommandLine because
	// importing net/http/httptest registers undesired flags on the latter.
	commandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// commands contains all registered subcommands.
	commands commander
)

var (
	configPath = commandLine.String("config", "docsite.json"+string(os.PathListSeparator)+filepath.Join("doc", "docsite.json"), "search `paths` for docsite JSON config (see https://github.com/sourcegraph/docsite#site-data)")
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("")
	fmt.Println("Hello, World!")
	commands.run(commandLine, "docsite", usage, os.Args[1:])
}

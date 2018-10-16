package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"
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
	// commandLine is the global flag set. It is used instead of flag.CommandLine because
	// importing net/http/httptest registers undesired flags on the latter.
	commandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// commands contains all registered subcommands.
	commands commander
)

var (
	configPath = commandLine.String("config", "docsite.json"+string(os.PathListSeparator)+filepath.Join("doc", "docsite.json"), "search `paths` for docsite JSON config")
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("")
	commands.run(commandLine, "docsite", usage, os.Args[1:])
}

func siteFromFlags() (*docsite.Site, error) {
	paths := filepath.SplitList(*configPath)
	for _, path := range paths {
		data, err := ioutil.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, errors.WithMessage(err, "reading docsite config file (from -config flag)")
		}
		return docsite.Open(data)
	}
	return nil, fmt.Errorf("no docsite.json config file found (search paths: %s)", *configPath)
}

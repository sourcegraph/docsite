package main

import (
	"flag"
	"log"
	"net"
	"net/http"
)

func init() {
	flagSet := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		httpAddr = flagSet.String("http", ":5080", "HTTP listen address for previewing")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)

		host, port, err := net.SplitHostPort(*httpAddr)
		if err != nil {
			return err
		}
		if host == "" {
			host = "0.0.0.0"
		}

		log.Printf("# Doc site is available at http://%s:%s", host, port)
		site, err := siteFromFlags()
		if err != nil {
			return err
		}
		return http.ListenAndServe(*httpAddr, site.Handler())
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "start a web server to serve the doc site",
		LongDescription:  "The serve subcommand starts a web server to serve the site over HTTP. After changing a source (Markdown) or template file, changes are immediately visible after reloading the page.",
		handler:          handler,
	})
}

package main

import (
	"flag"
	"log"
	"net/http"
)

func init() {
	flagSet := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		httpAddr = flagSet.String("http", ":8000", "HTTP listen address for previewing")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)
		log.Println("# Preview HTTP server listening on", *httpAddr)
		site := siteFromFlags()
		return http.ListenAndServe(*httpAddr, site.Handler())
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "serve a live preview of the site",
		LongDescription:  "The serve subcommand serves a live preview of the site over HTTP. After changing a source (Markdown) or template file, changes are immediately visible after reloading the page.",
		handler:          handler,
	})
}

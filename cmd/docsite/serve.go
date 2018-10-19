package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

func init() {
	flagSet := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		httpAddr       = flagSet.String("http", ":5080", "HTTP listen address for previewing")
		autocertDomain = flagSet.String("autocert-domain", "", "enable TLS listener and autocert for this domain name")
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

		site, _, err := siteFromFlags()
		if err != nil {
			return err
		}

		handler := site.Handler()
		l, err := net.Listen("tcp", *httpAddr)
		if err != nil {
			return err
		}
		if *autocertDomain != "" {
			log.Printf("# TLS listener enabled, Let's Encrypt domain %s", *autocertDomain)
			m := &autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(*autocertDomain),
			}
			l = tls.NewListener(l, m.TLSConfig())
			handler = m.HTTPHandler(handler)
		}
		log.Printf("# Doc site is available at http://%s:%s", host, port)
		return http.Serve(l, handler)
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "start a web server to serve the doc site",
		LongDescription:  "The serve subcommand starts a web server to serve the site over HTTP. After changing a source (Markdown) or template file, changes are immediately visible after reloading the page.",
		handler:          handler,
	})
}

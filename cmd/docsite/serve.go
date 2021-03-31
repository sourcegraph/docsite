package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net"
	"net/http"
	"sync"
)

func init() {
	flagSet := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		httpAddr    = flagSet.String("http", ":5080", "HTTP listen address for previewing")
		tlsCertPath = flagSet.String("tls-cert", "", "path to TLS certificate file")
		tlsKeyPath  = flagSet.String("tls-key", "", "path to TLS key file")
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

		var (
			handlerMu sync.Mutex
			handler   http.Handler
		)
		handlerMu.Lock()
		go func() {
			defer handlerMu.Unlock()
			site, _, err := siteFromFlags()
			if err != nil {
				log.Fatal(err)
			}
			handler = site.Handler()
		}()

		l, err := net.Listen("tcp", *httpAddr)
		if err != nil {
			return err
		}
		if *tlsCertPath != "" || *tlsKeyPath != "" {
			log.Printf("# TLS listener enabled")
			tlsCert, err := os.ReadFile(*tlsCertPath)
			if err != nil {
				return err
			}
			tlsKey, err := os.ReadFile(*tlsKeyPath)
			if err != nil {
				return err
			}
			cert, err := tls.X509KeyPair(tlsCert, tlsKey)
			if err != nil {
				return err
			}
			l = tls.NewListener(l, &tls.Config{
				NextProtos:   []string{"h2", "http/1.1"},
				Certificates: []tls.Certificate{cert},
			})
		}
		log.Printf("# Doc site is available at http://%s:%s", host, port)
		return http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Listen on the HTTP port immediately (and wait for the site to be loaded before sending an
			// HTTP response), instead of waiting to listen until the site is ready (which would cause
			// clients to immediately hang up).
			handlerMu.Lock()
			h := handler
			handlerMu.Unlock()
			h.ServeHTTP(w, r)
		}))
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "start a web server to serve the doc site",
		LongDescription:  "The serve subcommand starts a web server to serve the site over HTTP. After changing a source (Markdown) or template file, changes are immediately visible after reloading the page.",
		handler:          handler,
	})
}

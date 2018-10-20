package main

import (
	"crypto/tls"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
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

		site, _, err := siteFromFlags()
		if err != nil {
			return err
		}

		handler := site.Handler()
		l, err := net.Listen("tcp", *httpAddr)
		if err != nil {
			return err
		}
		if *tlsCertPath != "" || *tlsKeyPath != "" {
			log.Printf("# TLS listener enabled")
			tlsCert, err := ioutil.ReadFile(*tlsCertPath)
			if err != nil {
				return err
			}
			tlsKey, err := ioutil.ReadFile(*tlsKeyPath)
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

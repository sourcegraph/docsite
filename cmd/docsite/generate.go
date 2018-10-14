package main

import (
	"flag"
	"log"
)

func init() {
	flagSet := flag.NewFlagSet("generate", flag.ExitOnError)
	var (
		outDir = flagSet.String("out", "out", "path to output `dir` where .html files are written")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)
		log.Println(*outDir) // TODO!(sqs)
		return nil
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "write all output .html files for site",
		LongDescription:  "The generate subcommand generates all output .html files for the site and writes them to a directory.",
		aliases:          []string{"gen"},
		handler:          handler,
	})
}

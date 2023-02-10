package main

import (
	"context"
	"flag"
	"fmt"
)

func init() {
	flagSet := flag.NewFlagSet("ls", flag.ExitOnError)
	var (
		contentVersion = flagSet.String("content-version", "", "version of content to check")
	)

	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		site, _, err := siteFromFlags()
		if err != nil {
			return err
		}

		ctx := context.Background()
		pages, err := site.AllContentPages(ctx, *contentVersion)
		if err != nil {
			return err
		}

		for _, page := range pages {
			fmt.Printf("%s\n", page.FilePath)
		}
		return nil
	}

	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "list all documents",
		LongDescription:  "The ls subcommand lists all documents.",
		handler:          handler,
	})
}

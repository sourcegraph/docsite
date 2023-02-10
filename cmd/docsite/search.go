package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func init() {
	flagSet := flag.NewFlagSet("search", flag.ExitOnError)
	var (
		contentVersion = flagSet.String("content-version", "", "version of content to search")
	)

	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		site, _, err := siteFromFlags()
		if err != nil {
			return err
		}
		query := strings.Join(flagSet.Args(), " ")
		result, err := site.Search(context.Background(), *contentVersion, query)
		if err != nil {
			return err
		}
		if result.Total == 0 {
			log.Println("no results found")
			os.Exit(1)
		}
		for _, dr := range result.DocumentResults {
			for _, sr := range dr.SectionResults {
				var moreExcerpts string
				if len(sr.Excerpts) >= 2 {
					moreExcerpts = fmt.Sprintf(" (+%d more)", len(sr.Excerpts)-1)
				}
				if len(sr.Excerpts) > 0 {
					fmt.Printf("%s#%s: %s%s\n", dr.ID, sr.ID, sr.Excerpts[0], moreExcerpts)
				}
			}
		}
		return nil
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "search documents",
		LongDescription:  "The search subcommand searches documents to find matches of the query.",
		handler:          handler,
	})
}

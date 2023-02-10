package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func init() {
	flagSet := flag.NewFlagSet("check", flag.ExitOnError)
	var (
		contentVersion = flagSet.String("content-version", "", "version of content to check")
	)

	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		site, _, err := siteFromFlags()
		if err != nil {
			return err
		}
		problems, err := site.Check(context.Background(), *contentVersion)
		if err != nil {
			return err
		}
		if len(problems) > 0 {
			for _, problem := range problems {
				_, _ = fmt.Fprintln(os.Stderr, problem)
			}
			return fmt.Errorf("%d problems found", len(problems))
		}
		return nil
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "check all source files for problems",
		LongDescription:  "The check subcommand checks all source files for problems, such as template execution errors and broken links.",
		handler:          handler,
	})
}

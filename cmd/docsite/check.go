package main

import (
	"flag"
	"fmt"
)

func init() {
	flagSet := flag.NewFlagSet("check", flag.ExitOnError)

	handler := func(args []string) error {
		flagSet.Parse(args)
		site, err := siteFromFlags()
		if err != nil {
			return err
		}
		problems, err := site.Check()
		if err != nil {
			return err
		}
		if len(problems) > 0 {
			for _, problem := range problems {
				fmt.Println(problem)
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

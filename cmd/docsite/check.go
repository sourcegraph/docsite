package main

import (
	"flag"
	"fmt"
	"regexp"
)

func init() {
	flagSet := flag.NewFlagSet("check", flag.ExitOnError)
	var (
		// TODO(sqs): do not skip "^#" URLs; instead, check them.
		skipURLs = flagSet.String("skip-urls", "", "regexp `pattern` for URLs to skip in broken link check")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)

		var problems []string

		var skipURLsPattern *regexp.Regexp
		if *skipURLs != "" {
			var err error
			skipURLsPattern, err = regexp.Compile(*skipURLs)
			if err != nil {
				return err
			}
		}
		skipURL := func(url string) bool {
			return skipURLsPattern != nil && skipURLsPattern.MatchString(url)
		}

		// Check content.
		site := siteFromFlags()
		problems, err := site.Check(skipURL)
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

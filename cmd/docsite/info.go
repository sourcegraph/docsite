package main

import (
	"encoding/json"
	"flag"
	"fmt"
)

func init() {
	flagSet := flag.NewFlagSet("info", flag.ExitOnError)

	handler := func(args []string) error {
		flagSet.Parse(args)
		_, conf, err := siteFromFlags()
		if err != nil {
			return err
		}

		confJSON, err := json.MarshalIndent(conf, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(confJSON))
		return nil
	}

	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "print docsite configuration",
		LongDescription:  "The info subcommand prints information about the configured docsite.",
		handler:          handler,
	})
}

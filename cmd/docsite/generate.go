package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func init() {
	flagSet := flag.NewFlagSet("generate", flag.ExitOnError)
	var (
		outDir = flagSet.String("out", "out", "path to output `dir` where .html files are written")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)

		// Remove old output dir.
		if err := os.RemoveAll(*outDir); err != nil && !os.IsNotExist(err) {
			return errors.WithMessage(err, "removing old output dir")
		}

		// Generate .html files.
		gen := generatorFromFlags()
		generateFile := func(path string) error {
			if skipFile(path) {
				return nil
			}
			data, err := gen.Generate(path, true)
			if err != nil {
				return err
			}
			outPath := filepath.Join(*outDir, strings.TrimSuffix(path, ".md")+".html")
			if err := os.MkdirAll(filepath.Dir(outPath), 0700); err != nil {
				return err
			}
			return ioutil.WriteFile(outPath, data, 0600)
		}
		if err := walkFileSystem(gen.Sources, generateFile); err != nil {
			return err
		}

		// Copy assets.
		outAssetsPath := filepath.Join(*outDir, assetsURLPathComponent)
		if err := exec.Command("cp", "-R", *assetsDir, outAssetsPath).Run(); err != nil {
			return errors.WithMessage(err, "copying assets")
		}

		log.Printf("# Wrote site files to %s", *outDir)
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

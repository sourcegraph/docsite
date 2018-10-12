package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"

	"github.com/sourcegraph/docsite"
)

func init() {
	flagSet := flag.NewFlagSet("build", flag.ExitOnError)
	executable, _ := os.Executable()
	var (
		docsiteProg = flagSet.String("prog", executable, "path to docsite ELF executable to bundle site data into")
		outProg     = flagSet.String("o", "docsite-build", "output path for executable (containing bundled site data)")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)

		if *docsiteProg == "" {
			return usageError{errors.New("must provide -prog")}
		}

		site, config, err := siteFromFlags()
		if err != nil {
			return err
		}

		if config.IsELF {
			return errors.New("refusing to build executable from an already built executable")
		}

		tmpDir, err := ioutil.TempDir("", "docsite-build")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		// Create ELF sections with site data.
		var elfSections []string
		createELFSection := func(fs http.FileSystem, name string) error {
			path := filepath.Join(tmpDir, name+".zip")
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			if err := createZipFromFileSystem(fs, f); err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			elfSections = append(elfSections, fmt.Sprintf("docsite_%s=%s", name, path))
			return nil
		}
		if err := createELFSection(site.Assets, "assets"); err != nil {
			return err
		}
		if err := createELFSection(site.Templates, "templates"); err != nil {
			return err
		}
		if err := createELFSection(site.Content, "content"); err != nil {
			return err
		}

		// Add ELF section with docsite.json.
		config.IsELF = true
		configData, err := json.Marshal(config)
		if err != nil {
			return err
		}
		configPath := filepath.Join(tmpDir, "docsite.json")
		if err := ioutil.WriteFile(configPath, configData, 0600); err != nil {
			return err
		}
		elfSections = append(elfSections, fmt.Sprintf("docsite_config=%s", configPath))

		cmd := exec.Command("objcopy")
		for _, elfSection := range elfSections {
			cmd.Args = append(cmd.Args, "--add-section", elfSection)
		}
		cmd.Args = append(cmd.Args, *docsiteProg, *outProg)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %s\n\n%s", cmd.Args, err, out)
		}

		log.Printf("# Built executable with bundled site data: %s", *outProg)
		return nil
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "build a standalone executable with all files for a site",
		LongDescription:  "The build subcommand builds a standalone, statically linked executable that contains all content, assets, and templates for a site. The executable can be used to serve the site.",
		handler:          handler,
	})
}

func createZipFromFileSystem(fs http.FileSystem, w io.Writer) error {
	z := zip.NewWriter(w)
	created := map[string]struct{}{}
	err := docsite.WalkFileSystem(fs, func(path string) error {
		// Create dir if needed.
		dir := pathpkg.Dir(path)
		if _, ok := created[dir]; !ok {
			if _, err := z.Create(dir + "/"); err != nil {
				return err
			}
			created[dir] = struct{}{}
		}

		// Write file.
		w, err := z.Create(path)
		if err != nil {
			return err
		}
		f, err := fs.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		return err
	})
	if err == nil {
		err = z.Close()
	}
	return err
}

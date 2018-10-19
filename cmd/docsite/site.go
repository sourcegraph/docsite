package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"github.com/sourcegraph/docsite"
	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/zipfs"
)

func siteFromFlags() (*docsite.Site, *docsiteConfig, error) {
	// Check if env vars are set that refer to site data in external URLs.
	site, config, err := openDocsiteFromEnv()
	if site != nil || err != nil {
		return site, config, err
	}

	paths := filepath.SplitList(*configPath)
	for _, path := range paths {
		data, err := ioutil.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, nil, errors.WithMessage(err, "reading docsite config file (from -config flag)")
		}
		return openDocsiteFromConfig(data)
	}
	return nil, nil, fmt.Errorf("no docsite.json config file found (search paths: %s)", *configPath)
}

// docsiteConfig is the shape of docsite.json.
type docsiteConfig struct {
	Templates         string
	Content           string
	BaseURLPath       string
	Assets            string
	AssetsBaseURLPath string
	Check             struct {
		IgnoreURLPattern string
	}
}

func partialSiteFromConfig(config docsiteConfig) (*docsite.Site, error) {
	var checkIgnoreURLPattern *regexp.Regexp
	if config.Check.IgnoreURLPattern != "" {
		var err error
		checkIgnoreURLPattern, err = regexp.Compile(config.Check.IgnoreURLPattern)
		if err != nil {
			return nil, err
		}
	}

	return &docsite.Site{
		Base:                  &url.URL{Path: config.BaseURLPath},
		AssetsBase:            &url.URL{Path: config.AssetsBaseURLPath},
		CheckIgnoreURLPattern: checkIgnoreURLPattern,
	}, nil
}

// openDocsiteFromConfig reads the documentation site data from a docsite.json file.
func openDocsiteFromConfig(configData []byte) (*docsite.Site, *docsiteConfig, error) {
	var config docsiteConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, nil, errors.WithMessage(err, "reading docsite configuration")
	}

	site, err := partialSiteFromConfig(config)
	if err != nil {
		return nil, nil, err
	}

	httpDirOrNil := func(dir string) http.FileSystem {
		if dir == "" {
			return nil
		}
		return http.Dir(dir)
	}
	site.Templates = httpDirOrNil(config.Templates)
	site.Content = httpDirOrNil(config.Content)
	site.Assets = httpDirOrNil(config.Assets)
	return site, &config, nil
}

// openDocsiteFromConfig reads the documentation site data from env vars that refer to repositories.
func openDocsiteFromEnv() (*docsite.Site, *docsiteConfig, error) {
	configData := os.Getenv("DOCSITE_CONFIG")
	if configData == "" {
		return nil, nil, nil
	}

	var config docsiteConfig
	if err := json.Unmarshal([]byte(configData), &config); err != nil {
		return nil, nil, errors.WithMessage(err, "reading docsite configuration")
	}

	// Read site data.
	log.Println("# Downloading site data...")
	zipFileSystem := func(urlStr string) (http.FileSystem, error) {
		url, err := url.Parse(urlStr)
		if err != nil {
			return nil, err
		}

		resp, err := http.Get(urlStr)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		log.Printf("# Downloaded %s (%d bytes)", urlStr, len(body))
		z, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return nil, err
		}
		return prefixFileSystem{
			fs:     httpfs.New(zipfs.New(&zip.ReadCloser{Reader: *z}, urlStr)),
			prefix: "/" + url.Fragment,
		}, nil
	}
	assets, err := zipFileSystem(config.Assets)
	if err != nil {
		return nil, nil, err
	}
	templates, err := zipFileSystem(config.Templates)
	if err != nil {
		return nil, nil, err
	}
	content, err := zipFileSystem(config.Content)
	if err != nil {
		return nil, nil, err
	}

	site, err := partialSiteFromConfig(config)
	if err != nil {
		return nil, nil, err
	}
	site.Templates = templates
	site.Content = content
	site.Assets = assets
	return site, &config, nil
}

type prefixFileSystem struct {
	fs     http.FileSystem
	prefix string
}

func (fs prefixFileSystem) Open(name string) (http.File, error) {
	return fs.fs.Open(fs.prefix + name)
}

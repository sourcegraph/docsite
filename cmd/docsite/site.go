package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sourcegraph/docsite"
	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
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
	site.Content = nonVersionedFileSystem{httpDirOrNil(config.Content)}
	site.Assets = httpDirOrNil(config.Assets)
	return site, &config, nil
}

type nonVersionedFileSystem struct{ http.FileSystem }

func (fs nonVersionedFileSystem) OpenVersion(_ context.Context, version string) (http.FileSystem, error) {
	if version != "" {
		return nil, errors.New("content versioning is not supported")
	}
	return fs.FileSystem, nil
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
	assets, err := zipFileSystemFromURLWithDirFragment(config.Assets)
	if err != nil {
		return nil, nil, err
	}
	templates, err := zipFileSystemFromURLWithDirFragment(config.Templates)
	if err != nil {
		return nil, nil, err
	}

	// Content is in a versioned file system.
	content := &versionedFileSystemURL{url: config.Content}
	// Prefetch content at its default version. This ensures that the program exits if the content
	// default version is unavailable.
	if _, err := content.OpenVersion(context.Background(), ""); err != nil {
		return nil, nil, errors.WithMessage(err, "downloading content default version")
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

type versionedFileSystemURL struct {
	url string

	mu    sync.Mutex
	cache map[string]http.FileSystem
}

func (fs *versionedFileSystemURL) OpenVersion(ctx context.Context, version string) (http.FileSystem, error) {
	// HACK(sqs): this works for codeload.github.com
	if version == "" {
		version = "HEAD"
	}
	if strings.Contains(version, "..") || strings.Contains(version, "?") || strings.Contains(version, "#") {
		return nil, fmt.Errorf("invalid version %q", version)
	}

	fs.mu.Lock()
	if fs.cache == nil {
		fs.cache = map[string]http.FileSystem{}
	}
	vfs, ok := fs.cache[version]
	fs.mu.Unlock()
	if ok {
		return vfs, nil
	}

	urlStr := strings.Replace(fs.url, "$VERSION", version, -1)
	vfs, err := zipFileSystemFromURLWithDirFragment(urlStr)
	if err != nil {
		return nil, err
	}
	fs.mu.Lock()
	fs.cache[version] = vfs
	fs.mu.Unlock()
	return vfs, nil
}

func zipFileSystemFromURLWithDirFragment(urlStr string) (http.FileSystem, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	dir := url.Fragment
	url.Fragment = ""
	return zipFileSystemAtURL(url.String(), dir)
}

func zipFileSystemAtURL(url, dir string) (http.FileSystem, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Printf("# Downloaded %s (%d bytes)", url, len(body))
	z, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, err
	}

	// Expand "*/" dir prefix to the actual dir name, if any. This is because GitHub codeload Zip
	// archives have a top-level directory that is $REPO-$REV, where $REV is the sanitized rev
	// (replacing '/' with '-', for example), and we just want to chop off the first dir.
	if strings.HasPrefix(dir, "*/") && len(z.File) > 0 {
		dir = z.File[0].Name + strings.TrimPrefix(dir, "*/")
	}

	// Keep only the files actually needed, to reduce memory usage.
	m := map[string]string{}
	for _, fh := range z.File {
		if strings.HasPrefix(fh.Name, dir) && !strings.HasSuffix(fh.Name, "/") {
			f, err := fh.Open()
			if err != nil {
				return nil, errors.WithMessage(err, fmt.Sprintf("open %q", fh.Name))
			}
			data, err := ioutil.ReadAll(f)
			f.Close()
			if err != nil {
				return nil, errors.WithMessage(err, fmt.Sprintf("read %q", fh.Name))
			}
			name := strings.TrimPrefix(fh.Name, dir)
			m[name] = string(data)
		}
	}
	body = nil
	z = nil

	return httpfs.New(mapfs.New(m)), nil
}
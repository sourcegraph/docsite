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
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

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
		return openDocsiteFromConfig(data, filepath.Dir(path))
	}
	return nil, nil, fmt.Errorf("no docsite.json config file found (search paths: %s)", *configPath)
}

// docsiteConfig is the shape of docsite.json.
//
// See ["Site data" in README.md](../../README.md#site-data) for documentation on this type's
// fields.
type docsiteConfig struct {
	Content               string
	ContentExcludePattern string
	BaseURLPath           string
	Templates             string
	Assets                string
	AssetsBaseURLPath     string
	Check                 struct {
		IgnoreURLPattern string
	}
}

func partialSiteFromConfig(config docsiteConfig) (*docsite.Site, error) {
	var site docsite.Site
	if config.ContentExcludePattern != "" {
		var err error
		site.ContentExcludePattern, err = regexp.Compile(config.ContentExcludePattern)
		if err != nil {
			return nil, err
		}
	}
	if config.Check.IgnoreURLPattern != "" {
		var err error
		site.CheckIgnoreURLPattern, err = regexp.Compile(config.Check.IgnoreURLPattern)
		if err != nil {
			return nil, err
		}
	}
	if config.BaseURLPath != "" {
		site.Base = &url.URL{Path: config.BaseURLPath}
	}
	if config.AssetsBaseURLPath != "" {
		site.AssetsBase = &url.URL{Path: config.AssetsBaseURLPath}
	}
	return &site, nil
}

// openDocsiteFromConfig reads the documentation site data from a docsite.json file. All file system
// paths in docsite.json are resolved relative to baseDir.
func openDocsiteFromConfig(configData []byte, baseDir string) (*docsite.Site, *docsiteConfig, error) {
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
		return http.Dir(filepath.Join(baseDir, dir))
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

// openDocsiteFromEnv reads the documentation site data from env vars that refer to repositories.
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
	cache map[string]fileSystemCacheEntry
}

type fileSystemCacheEntry struct {
	fs http.FileSystem
	at time.Time
}

const fileSystemCacheTTL = 5 * time.Minute

func (fs *versionedFileSystemURL) OpenVersion(ctx context.Context, version string) (http.FileSystem, error) {
	// HACK(sqs): this works for codeload.github.com
	if version == "" {
		// HACK: Use master instead of HEAD even though master is technically incorrect in the
		// general case. This is because we require that $VERSION be interpolated into
		// refs/heads/$VERSION not just $VERSION (to avoid the security problem described below),
		// and refs/heads/HEAD doesn't work in general.
		version = "master"
	}
	if strings.Contains(version, "..") || strings.Contains(version, "?") || strings.Contains(version, "#") {
		return nil, fmt.Errorf("invalid version %q", version)
	}

	fs.mu.Lock()
	if fs.cache == nil {
		fs.cache = map[string]fileSystemCacheEntry{}
	}
	e, ok := fs.cache[version]
	if ok && time.Since(e.at) > fileSystemCacheTTL {
		log.Printf("# Cached site data expired after %s, will re-download", fileSystemCacheTTL)
		delete(fs.cache, version)
		ok = false
	}
	fs.mu.Unlock()
	if ok {
		return e.fs, nil
	}

	urlStr := fs.url
	if strings.Contains(urlStr, "$VERSION") && strings.Contains(urlStr, "github") && !strings.Contains(urlStr, "refs/heads/$VERSION") {
		return nil, fmt.Errorf("refusing to use insecure docsite configuration for multi-version-aware GitHub URLs: the URL pattern %q must include \"refs/heads/$VERSION\", not just \"$VERSION\" (see docsite README.md for more information)", urlStr)
	}
	urlStr = strings.Replace(fs.url, "$VERSION", version, -1)

	// HACK: Workaround for https://github.com/sourcegraph/sourcegraph/issues/3030. This assumes
	// that tags all begin with "vN" where N is some number.
	if len(version) >= 2 && version[0] == 'v' && unicode.IsDigit(rune(version[1])) {
		urlStr = strings.Replace(urlStr, "refs/heads/", "refs/tags/", 1)
	}

	vfs, err := zipFileSystemFromURLWithDirFragment(urlStr)
	if err != nil {
		return nil, err
	}
	fs.mu.Lock()
	fs.cache[version] = fileSystemCacheEntry{fs: vfs, at: time.Now()}
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
	if resp.StatusCode == http.StatusNotFound {
		return nil, &os.PathError{Op: "Get", Path: url, Err: os.ErrNotExist}
	} else if resp.StatusCode != http.StatusOK {
		return nil, &os.PathError{Op: "Get", Path: url, Err: fmt.Errorf("HTTP response status code %d", resp.StatusCode)}
	}
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
	m, err := mapFromZipArchive(z, dir)
	if err != nil {
		return nil, err
	}
	body = nil
	z = nil

	return httpfs.New(mapfs.New(m)), nil
}

// mapFromZipArchive adds the contents of all files in the Zip archive (in dir) to the map.
func mapFromZipArchive(z *zip.Reader, dir string) (map[string]string, error) {
	readFileHeader := func(zf *zip.File) ([]byte, error) {
		f, err := zf.Open()
		if err != nil {
			return nil, errors.WithMessagef(err, "open %q", zf.Name)
		}
		data, err := ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, errors.WithMessagef(err, "read %q", zf.Name)
		}
		return data, nil
	}
	readFile := func(path string) ([]byte, error) {
		for _, f := range z.File {
			if f.Name == path {
				return readFileHeader(f)
			}
		}
		return nil, &os.PathError{Op: "readFile (in zip archive)", Path: path, Err: os.ErrNotExist}
	}

	m := map[string]string{}
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, dir) && !strings.HasSuffix(f.Name, "/") {
			data, err := readFileHeader(f)
			if err != nil {
				return nil, err
			}

			// Dereference symlinks.
			if f.Mode()&os.ModeSymlink != 0 {
				targetPath := path.Join(path.Dir(f.Name), string(data))
				data, err = readFile(targetPath)
				if err != nil {
					return nil, errors.WithMessagef(err, "dereferencing symlink at %q", f.Name)
				}
			}

			name := strings.TrimPrefix(f.Name, dir)
			m[name] = string(data)
		}
	}
	return m, nil
}

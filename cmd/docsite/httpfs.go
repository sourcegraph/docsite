package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// cachedFileSystem wraps and caches an http.FileSystem.
type cachedFileSystem struct {
	fetch func() (http.FileSystem, error)

	mu      sync.Mutex
	fs      http.FileSystem
	refresh *sync.Once
	at      time.Time
}

func newCachedFileSystem(fetch func() (http.FileSystem, error)) *cachedFileSystem {
	return &cachedFileSystem{fetch: fetch, refresh: new(sync.Once)}
}

func (c *cachedFileSystem) fetchAndCache() error {
	fs, err := c.fetch()
	if err != nil {
		fs = nil
	}
	c.mu.Lock()
	c.store(fs)
	c.mu.Unlock()
	return err
}

func (c *cachedFileSystem) store(fs http.FileSystem) {
	c.fs = fs
	c.refresh = new(sync.Once) // reset sync.Once so it can be refreshed next time when needed
	c.at = time.Now()
}

func (c *cachedFileSystem) get() (http.FileSystem, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fs := c.fs
	var err error
	if fs != nil && time.Since(c.at) > fileSystemCacheTTL {
		log.Printf("# Cached template/asset data expired after %s, refreshing in background", fileSystemCacheTTL)
		go c.refresh.Do(func() {
			if err := c.fetchAndCache(); err != nil {
				log.Printf("# Error refreshing template/asset data in background: %s", err)
			}
		})
	} else if fs == nil {
		fs, err = c.fetch()
	}
	return fs, err
}

func (c *cachedFileSystem) Open(name string) (http.File, error) {
	httpfs, err := c.get()
	if err != nil {
		return nil, err
	}
	return httpfs.Open(name)
}

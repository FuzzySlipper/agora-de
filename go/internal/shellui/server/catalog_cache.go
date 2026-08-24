package server

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agora-de.local/go/internal/appcatalog"
	"agora-de.local/go/internal/shellui/catalog"
)

const catalogRevalidateInterval = time.Second

type catalogSnapshot struct {
	catalog   *appcatalog.Catalog
	apps      []catalog.AppView
	iconFiles map[string]string
}

type catalogCache struct {
	mu              sync.Mutex
	config          Config
	snapshot        catalogSnapshot
	desktopRevision uint64
	checkedAt       time.Time
	revalidateAfter time.Duration
	now             func() time.Time
}

func newCatalogCache(config Config) (*catalogCache, error) {
	snapshot, err := loadCatalogSnapshot(config)
	if err != nil {
		return nil, err
	}
	now := time.Now
	cache := &catalogCache{
		config:          config,
		snapshot:        snapshot,
		checkedAt:       now(),
		revalidateAfter: catalogRevalidateInterval,
		now:             now,
	}
	if strings.TrimSpace(config.CatalogProvider) == CatalogProviderDesktopEntries {
		cache.desktopRevision, err = appcatalog.DesktopEntryRevision(config.DesktopEntryRoots...)
		if err != nil {
			return nil, err
		}
	}
	return cache, nil
}

func loadCatalogSnapshot(config Config) (catalogSnapshot, error) {
	appCatalog, err := catalogSource(config)
	if err != nil {
		return catalogSnapshot{}, err
	}
	apps, err := launchAwareAppViews(config, appCatalog)
	if err != nil {
		return catalogSnapshot{}, err
	}
	iconFiles := map[string]string{}
	apps = catalog.ApplyIconURLs(apps, iconResolver(config), func(path string) string {
		key := iconKey(path)
		iconFiles[key] = path
		return CatalogIconPathPrefix + key + "/" + filepath.Base(path)
	})
	return catalogSnapshot{catalog: appCatalog, apps: apps, iconFiles: iconFiles}, nil
}

func (cache *catalogCache) current() catalogSnapshot {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if strings.TrimSpace(cache.config.CatalogProvider) != CatalogProviderDesktopEntries {
		return cache.snapshot
	}
	now := cache.now()
	if now.Sub(cache.checkedAt) < cache.revalidateAfter {
		return cache.snapshot
	}
	cache.checkedAt = now
	revision, err := appcatalog.DesktopEntryRevision(cache.config.DesktopEntryRoots...)
	if err != nil || revision == cache.desktopRevision {
		return cache.snapshot
	}
	next, err := loadCatalogSnapshot(cache.config)
	if err != nil {
		return cache.snapshot
	}
	cache.snapshot = next
	cache.desktopRevision = revision
	return cache.snapshot
}

func (cache *catalogCache) appsProvider(*http.Request) ([]catalog.AppView, error) {
	current := cache.current()
	return append([]catalog.AppView(nil), current.apps...), nil
}

func (cache *catalogCache) entry(id string) (appcatalog.Entry, bool) {
	return cache.current().catalog.Get(id)
}

func (cache *catalogCache) iconPath(key string) (string, bool) {
	path, ok := cache.current().iconFiles[key]
	return path, ok
}

package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCatalogCacheRevalidatesDesktopEntryChanges(t *testing.T) {
	root := t.TempDir()
	writeServerDesktopEntry(t, root, "alpha.desktop", `[Desktop Entry]
Type=Application
Name=Alpha
Exec=/usr/bin/true
`)
	cache, err := newCatalogCache(Config{
		CatalogProvider:   CatalogProviderDesktopEntries,
		DesktopEntryRoots: []string{root},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	cache.now = func() time.Time { return now }
	cache.checkedAt = now

	apps, err := cache.appsProvider(nil)
	if err != nil || len(apps) != 1 || apps[0].Name != "Alpha" {
		t.Fatalf("initial apps=%+v err=%v", apps, err)
	}
	writeServerDesktopEntry(t, root, "beta.desktop", `[Desktop Entry]
Type=Application
Name=Beta
Exec=/usr/bin/true
`)
	apps, _ = cache.appsProvider(nil)
	if len(apps) != 1 {
		t.Fatalf("cache refreshed before bounded interval: %+v", apps)
	}

	now = now.Add(catalogRevalidateInterval)
	apps, _ = cache.appsProvider(nil)
	if len(apps) != 2 || apps[1].Name != "Beta" {
		t.Fatalf("added desktop entry was not revalidated: %+v", apps)
	}
	if _, ok := cache.entry("beta.desktop"); !ok {
		t.Fatal("launch catalog did not receive revalidated desktop entry")
	}

	if err := os.WriteFile(filepath.Join(root, "beta.desktop"), []byte("[Desktop Entry]\nType=Application\nName=Beta Changed\nExec=/usr/bin/true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now = now.Add(catalogRevalidateInterval)
	apps, _ = cache.appsProvider(nil)
	if len(apps) != 2 || apps[1].Name != "Beta Changed" {
		t.Fatalf("changed desktop entry was not revalidated: %+v", apps)
	}

	if err := os.Remove(filepath.Join(root, "alpha.desktop")); err != nil {
		t.Fatal(err)
	}
	now = now.Add(catalogRevalidateInterval)
	apps, _ = cache.appsProvider(nil)
	if len(apps) != 1 || apps[0].ID != "beta.desktop" {
		t.Fatalf("removed desktop entry remained in cache: %+v", apps)
	}
}

package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIconResolverResolvesAbsolutePath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolution := NewIconResolver(nil, nil).Resolve(path)
	if resolution.Path != path {
		t.Fatalf("path = %q, want %q", resolution.Path, path)
	}
}

func TestIconResolverResolvesPixmapName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample-app.png")
	if err := os.WriteFile(path, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolution := NewIconResolver(nil, []string{root}).Resolve("sample-app")
	if resolution.Path != path {
		t.Fatalf("path = %q, want %q", resolution.Path, path)
	}
}

func TestIconResolverResolvesThemeIcon(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hicolor", "scalable", "apps", "sample-app.svg")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolution := NewIconResolver([]string{root}, nil).Resolve("sample-app")
	if resolution.Path != path {
		t.Fatalf("path = %q, want %q", resolution.Path, path)
	}
}

func TestIconResolverRejectsMissingAndRelativePaths(t *testing.T) {
	resolver := NewIconResolver([]string{t.TempDir()}, []string{t.TempDir()})
	for _, icon := range []string{"missing-app", "relative/path"} {
		if got := resolver.Resolve(icon); got.Path != "" {
			t.Fatalf("icon %q resolved to %q, want empty", icon, got.Path)
		}
	}
}

func TestApplyIconURLsKeepsFallbackWhenUnresolved(t *testing.T) {
	views := []AppView{{ID: "missing.desktop", Icon: "missing"}}
	got := ApplyIconURLs(views, NewIconResolver(nil, nil), func(path string) string {
		t.Fatalf("url callback should not be called for %q", path)
		return ""
	})
	if got[0].IconURL != "" {
		t.Fatalf("icon url = %q, want empty", got[0].IconURL)
	}
}

func TestDefaultIconRootsIncludeUserAndSystemLocations(t *testing.T) {
	home := t.TempDir()
	themeRoots := strings.Join(DefaultIconThemeRoots(home), "\n")
	pixmapRoots := strings.Join(DefaultIconPixmapRoots(home), "\n")
	if !strings.Contains(themeRoots, filepath.Join(home, ".local/share/icons")) {
		t.Fatalf("theme roots missing user root: %s", themeRoots)
	}
	if !strings.Contains(pixmapRoots, filepath.Join(home, ".local/share/pixmaps")) {
		t.Fatalf("pixmap roots missing user root: %s", pixmapRoots)
	}
}

package staticserve

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolverDefaultsRootToIndex(t *testing.T) {
	root := t.TempDir()
	resolver, err := NewResolver(root)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolver.Resolve("/")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "index.html")
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
}

func TestResolverRejectsTraversalAndAbsolutePaths(t *testing.T) {
	resolver, err := NewResolver(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	rejected := []string{"../secret", "/etc/passwd", "assets/../../secret"}
	for _, requestPath := range rejected {
		if _, err := resolver.Resolve(requestPath); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("Resolve(%q) error = %v, want ErrUnsafePath", requestPath, err)
		}
	}
}

func TestResolverAllowsNestedAssets(t *testing.T) {
	root := t.TempDir()
	resolver, err := NewResolver(root)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolver.Resolve("assets/app.css")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "assets", "app.css")
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
}


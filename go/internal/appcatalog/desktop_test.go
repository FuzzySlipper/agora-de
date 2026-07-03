package appcatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDesktopEntryFixture(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "..", "harness", "fixtures", "appcatalog", "example-browser.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	entry, err := ParseDesktopEntry("example-browser.desktop", file)
	if err != nil {
		t.Fatal(err)
	}

	if entry.Name != "Example Browser" {
		t.Fatalf("name = %q, want Example Browser", entry.Name)
	}
	if entry.Exec != "example-browser --new-window %u" {
		t.Fatalf("exec = %q", entry.Exec)
	}
	if entry.Icon != "example-browser" {
		t.Fatalf("icon = %q, want example-browser", entry.Icon)
	}
	if entry.NoDisplay {
		t.Fatal("fixture should be visible")
	}
}

func TestParseDesktopEntryRequiresNameAndExec(t *testing.T) {
	_, err := ParseDesktopEntry("bad.desktop", strings.NewReader("[Desktop Entry]\nName=Bad\n"))
	if err == nil {
		t.Fatal("ParseDesktopEntry accepted missing Exec")
	}
}

func TestCatalogVisibleEntriesSkipsNoDisplay(t *testing.T) {
	catalog := NewCatalog()
	catalog.Add(Entry{ID: "visible", Name: "Visible", Exec: "visible"})
	catalog.Add(Entry{ID: "hidden", Name: "Hidden", Exec: "hidden", NoDisplay: true})

	entries := catalog.VisibleEntries()
	if len(entries) != 1 {
		t.Fatalf("visible entries = %d, want 1", len(entries))
	}
	if entries[0].ID != "visible" {
		t.Fatalf("visible entry id = %q, want visible", entries[0].ID)
	}
}


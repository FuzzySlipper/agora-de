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
	if got := strings.Join(entry.ExecTokens, " "); got != "example-browser --new-window" {
		t.Fatalf("exec tokens = %q, want field-code-stripped command", got)
	}
	if !entry.ExecSupported {
		t.Fatal("fixture exec should be supported")
	}
	if entry.Icon != "example-browser" {
		t.Fatalf("icon = %q, want example-browser", entry.Icon)
	}
	if entry.NoDisplay {
		t.Fatal("fixture should be visible")
	}
}

func TestParseDesktopEntryRequiresNameAndExec(t *testing.T) {
	_, err := ParseDesktopEntry("bad.desktop", strings.NewReader("[Desktop Entry]\nType=Application\nName=Bad\n"))
	if err == nil {
		t.Fatal("ParseDesktopEntry accepted missing Exec")
	}
}

func TestImportDesktopEntriesFiltersAndOrdersEntries(t *testing.T) {
	root := t.TempDir()
	writeDesktopEntry(t, root, "zeta.desktop", `[Desktop Entry]
Type=Application
Name=Zeta
Exec=zeta %u
Icon=zeta
`)
	writeDesktopEntry(t, root, "alpha.desktop", `[Desktop Entry]
Type=Application
Name=Alpha
Exec=alpha
Icon=alpha
`)
	writeDesktopEntry(t, root, "hidden.desktop", `[Desktop Entry]
Type=Application
Name=Hidden
Exec=hidden
Hidden=true
`)
	writeDesktopEntry(t, root, "nodisplay.desktop", `[Desktop Entry]
Type=Application
Name=No Display
Exec=no-display
NoDisplay=true
`)
	writeDesktopEntry(t, root, "directory.desktop", `[Desktop Entry]
Type=Directory
Name=Settings
`)
	writeDesktopEntry(t, root, "malformed.desktop", `[Desktop Entry]
Type=Application
Name=Malformed
`)

	catalog, err := ImportDesktopEntries(root)
	if err != nil {
		t.Fatal(err)
	}
	entries := catalog.VisibleEntries()
	if len(entries) != 2 {
		t.Fatalf("visible entries = %d, want 2: %+v", len(entries), entries)
	}
	if entries[0].ID != "alpha.desktop" || entries[1].ID != "zeta.desktop" {
		t.Fatalf("entries not sorted by name: %+v", entries)
	}
	if !entries[1].Launchable() {
		t.Fatalf("zeta should be launchable: %+v", entries[1])
	}
	if got := strings.Join(entries[1].ExecTokens, " "); got != "zeta" {
		t.Fatalf("zeta exec tokens = %q, want zeta", got)
	}
}

func TestImportDesktopEntriesUsesFirstRootPrecedence(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	writeDesktopEntry(t, first, "same.desktop", `[Desktop Entry]
Type=Application
Name=First
Exec=first
`)
	writeDesktopEntry(t, second, "same.desktop", `[Desktop Entry]
Type=Application
Name=Second
Exec=second
`)

	catalog, err := ImportDesktopEntries(first, second)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := catalog.Get("same.desktop")
	if !ok {
		t.Fatal("same.desktop not imported")
	}
	if entry.Name != "First" {
		t.Fatalf("entry name = %q, want First", entry.Name)
	}
}

func TestNormalizeExecHandlesCommonFieldCodesConservatively(t *testing.T) {
	tokens, ok := NormalizeExec(`"sample app" --open %U --literal=%% --name=%c`)
	if !ok {
		t.Fatal("NormalizeExec rejected supported field codes")
	}
	if got := strings.Join(tokens, "|"); got != "sample app|--open|--literal=%|--name=" {
		t.Fatalf("tokens = %q", got)
	}

	if _, ok := NormalizeExec("sample %Z"); ok {
		t.Fatal("NormalizeExec accepted unknown field code")
	}
	if _, ok := NormalizeExec(`"unterminated`); ok {
		t.Fatal("NormalizeExec accepted unterminated quote")
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

func writeDesktopEntry(t *testing.T, root string, name string, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

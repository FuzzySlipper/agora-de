package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectDefaultsToAgoraDefault(t *testing.T) {
	selection, err := Select(SelectionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if selection.Manifest.ID != DefaultThemeID {
		t.Fatalf("selected theme = %q, want %q", selection.Manifest.ID, DefaultThemeID)
	}
	if selection.Source != "builtin:"+DefaultThemeID {
		t.Fatalf("theme source = %q", selection.Source)
	}
	if !strings.Contains(selection.CSS, TokenEvidenceAccent) || !strings.Contains(selection.CSS, TokenBackground+": #0f172a;") {
		t.Fatalf("default theme CSS missing expected tokens: %s", selection.CSS)
	}
}

func TestSelectBundledVariant(t *testing.T) {
	selection, err := Select(SelectionOptions{ID: EmberThemeID})
	if err != nil {
		t.Fatal(err)
	}
	if selection.Manifest.ID != EmberThemeID {
		t.Fatalf("selected theme = %q, want %q", selection.Manifest.ID, EmberThemeID)
	}
	if selection.Manifest.Name != "Agora Ember" {
		t.Fatalf("selected theme name = %q", selection.Manifest.Name)
	}
	if !strings.Contains(selection.CSS, TokenAccent+": #fb923c;") {
		t.Fatalf("ember CSS missing accent: %s", selection.CSS)
	}
}

func TestSelectUserManifestOverlaysDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := os.WriteFile(path, []byte(`{"id":"user-theme","name":"User Theme","tokens":{"--agora-accent":"#38bdf8"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	selection, err := Select(SelectionOptions{ID: EmberThemeID, ManifestPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if selection.Manifest.ID != "user-theme" {
		t.Fatalf("selected theme = %q, want user-theme", selection.Manifest.ID)
	}
	if selection.Source != path {
		t.Fatalf("theme source = %q, want %q", selection.Source, path)
	}
	if selection.Manifest.Tokens[TokenAccent] != "#38bdf8" {
		t.Fatalf("accent = %q", selection.Manifest.Tokens[TokenAccent])
	}
	if selection.Manifest.Tokens[TokenPanelHeight] != DefaultTokens()[TokenPanelHeight] {
		t.Fatalf("default token overlay missing panel height: %+v", selection.Manifest.Tokens)
	}
}

func TestSelectRejectsUnknownBundledTheme(t *testing.T) {
	_, err := Select(SelectionOptions{ID: "missing-theme"})
	if err == nil {
		t.Fatal("Select accepted unknown bundled theme")
	}
}

func TestSelectRejectsInvalidUserManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := os.WriteFile(path, []byte(`{"id":"bad","name":"Bad","tokens":{"--other-bg":"red"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Select(SelectionOptions{ManifestPath: path})
	if err == nil {
		t.Fatal("Select accepted invalid user manifest")
	}
}

func TestEmberFixtureMatchesBundledManifest(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "..", "..", "harness", "fixtures", "theme", "agora-ember-theme.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	fixture, err := DecodeManifest(file)
	if err != nil {
		t.Fatal(err)
	}
	bundled, ok := BuiltinManifest(EmberThemeID)
	if !ok {
		t.Fatal("ember bundled manifest missing")
	}
	if fixture.ID != bundled.ID || fixture.Name != bundled.Name {
		t.Fatalf("fixture = %s/%s, bundled = %s/%s", fixture.ID, fixture.Name, bundled.ID, bundled.Name)
	}
	for name, value := range bundled.Tokens {
		if fixture.Tokens[name] != value {
			t.Fatalf("fixture token %s = %q, want %q", name, fixture.Tokens[name], value)
		}
	}
}

package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeManifestFixture(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "..", "..", "harness", "fixtures", "theme", "agora-default-theme.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	manifest, err := DecodeManifest(file)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "agora-default" {
		t.Fatalf("theme id = %q, want agora-default", manifest.ID)
	}
	if manifest.Name != "Agora Observatory" {
		t.Fatalf("theme name = %q, want Agora Observatory", manifest.Name)
	}
	if len(manifest.Tokens) != 3 {
		t.Fatalf("token count = %d, want 3", len(manifest.Tokens))
	}
}

func TestDecodeManifestRejectsNonAgoraToken(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader(`{"id":"bad","name":"Bad","tokens":{"--other-bg":"red"}}`))
	if err == nil {
		t.Fatal("DecodeManifest accepted non-agora token")
	}
}

func TestSafeTokenCSS(t *testing.T) {
	css, err := SafeTokenCSS(map[string]string{"--agora-bg": "#101418"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(css, "--agora-bg: #101418;") {
		t.Fatalf("generated css = %q", css)
	}
}

func TestValidateSafeVisualCSSRejectsLayoutAndExfiltration(t *testing.T) {
	rejected := []string{
		":root { position: fixed; }",
		"@import url(https://example.invalid/theme.css);",
		":root { --agora-bg: url(https://example.invalid/pixel); }",
	}
	for _, css := range rejected {
		if err := ValidateSafeVisualCSS(css); err == nil {
			t.Fatalf("ValidateSafeVisualCSS accepted %q", css)
		}
	}
}


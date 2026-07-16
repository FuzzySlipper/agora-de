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
	if manifest.Name != "Agora Tide" {
		t.Fatalf("theme name = %q, want Agora Tide", manifest.Name)
	}
	if len(manifest.Tokens) != len(DefaultTokens()) {
		t.Fatalf("token count = %d, want %d", len(manifest.Tokens), len(DefaultTokens()))
	}
	for name, value := range DefaultTokens() {
		if manifest.Tokens[name] != value {
			t.Fatalf("theme token %s = %q, want %q", name, manifest.Tokens[name], value)
		}
	}
}

func TestDecodeManifestRejectsNonAgoraToken(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader(`{"id":"bad","name":"Bad","tokens":{"--other-bg":"red"}}`))
	if err == nil {
		t.Fatal("DecodeManifest accepted non-agora token")
	}
}

func TestDecodeManifestRejectsUnknownAgoraToken(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader(`{"id":"bad","name":"Bad","tokens":{"--agora-not-real":"red"}}`))
	if err == nil {
		t.Fatal("DecodeManifest accepted unknown agora token")
	}
}

func TestDecodeManifestRejectsMissingTokens(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader(`{"id":"bad","name":"Bad","tokens":{}}`))
	if err == nil {
		t.Fatal("DecodeManifest accepted missing tokens")
	}
}

func TestSafeTokenCSS(t *testing.T) {
	css, err := SafeTokenCSS(map[string]string{"--agora-bg": "#101418", "--agora-accent": "#46b3a5"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(css, "--agora-accent") > strings.Index(css, "--agora-bg") {
		t.Fatalf("generated css should be deterministic and sorted: %q", css)
	}
	if !strings.Contains(css, "--agora-bg: #101418;") {
		t.Fatalf("generated css = %q", css)
	}
}

func TestDefaultTokenDefinitionsSeparateEvidenceMarkers(t *testing.T) {
	definitions := DefaultTokenDefinitions()
	if len(definitions) != len(DefaultTokens()) {
		t.Fatalf("definition count = %d, want %d", len(definitions), len(DefaultTokens()))
	}

	roles := map[string]TokenRole{}
	for _, definition := range definitions {
		roles[definition.Name] = definition.Role
	}
	for _, name := range []string{TokenEvidenceBackground, TokenEvidenceAccent, TokenEvidenceStrong} {
		if roles[name] != RoleEvidence {
			t.Fatalf("token %s role = %q, want %q", name, roles[name], RoleEvidence)
		}
	}
	if roles[TokenBackground] == RoleEvidence || roles[TokenAccent] == RoleEvidence {
		t.Fatalf("presentation tokens should not be classified as evidence markers: %+v", roles)
	}
	for _, name := range []string{
		TokenPanelControlHeight,
		TokenPanelBackground,
		TokenPanelShadow,
		TokenPopupShadow,
		TokenOverlayLabelBackground,
		TokenOverlayChipBackground,
		TokenFocusGlow,
		TokenTaskbarMinimizedBackground,
		TokenTaskbarMinimizedBorder,
	} {
		if roles[name] != RoleComponent {
			t.Fatalf("token %s role = %q, want %q", name, roles[name], RoleComponent)
		}
	}
	if css := MustDefaultTokenCSS(); !strings.Contains(css, TokenEvidenceAccent) || !strings.Contains(css, TokenPanelHeight) {
		t.Fatalf("default CSS missing centralized tokens: %s", css)
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

func TestValidateTokenRejectsUnsafeFragments(t *testing.T) {
	rejected := []string{
		"url(https://example.invalid/pixel)",
		"position: fixed",
		"@import https://example.invalid/theme.css",
	}
	for _, value := range rejected {
		if err := ValidateToken(TokenBackground, value); err == nil {
			t.Fatalf("ValidateToken accepted %q", value)
		}
	}
}

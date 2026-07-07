package theme

import (
	"fmt"
	"os"
	"strings"
)

const (
	DefaultThemeID = "agora-default"
	EmberThemeID   = "agora-ember"
)

type SelectionOptions struct {
	ID           string
	ManifestPath string
}

type Selection struct {
	Manifest Manifest
	CSS      string
	Source   string
}

func Select(options SelectionOptions) (Selection, error) {
	if path := strings.TrimSpace(options.ManifestPath); path != "" {
		file, err := os.Open(path)
		if err != nil {
			return Selection{}, fmt.Errorf("open theme manifest %q: %w", path, err)
		}
		defer file.Close()
		manifest, err := DecodeManifest(file)
		if err != nil {
			return Selection{}, err
		}
		return selectionFromManifest(manifest, path)
	}

	id := strings.TrimSpace(options.ID)
	if id == "" {
		id = DefaultThemeID
	}
	manifest, ok := BuiltinManifest(id)
	if !ok {
		return Selection{}, fmt.Errorf("unknown bundled theme %q", id)
	}
	return selectionFromManifest(manifest, "builtin:"+manifest.ID)
}

func MustDefaultSelection() Selection {
	selection, err := Select(SelectionOptions{ID: DefaultThemeID})
	if err != nil {
		panic(err)
	}
	return selection
}

func BuiltinManifests() []Manifest {
	return []Manifest{
		DefaultManifest(),
		EmberManifest(),
	}
}

func BuiltinManifest(id string) (Manifest, bool) {
	id = strings.TrimSpace(id)
	for _, manifest := range BuiltinManifests() {
		if manifest.ID == id {
			return cloneManifest(manifest), true
		}
	}
	return Manifest{}, false
}

func EmberManifest() Manifest {
	tokens := DefaultTokens()
	tokens[TokenBackground] = "#12100f"
	tokens[TokenForeground] = "#f1f5f9"
	tokens[TokenSurface] = "#1c1917"
	tokens[TokenSurfaceRaised] = "#292524"
	tokens[TokenSurfaceStrong] = "#0c0a09"
	tokens[TokenTextMuted] = "#a8a29e"
	tokens[TokenAccent] = "#fb923c"
	tokens[TokenWarning] = "#fde047"
	tokens[TokenBorder] = "#78716c"
	tokens[TokenBorderSubtle] = "#44403c"
	return Manifest{
		ID:     EmberThemeID,
		Name:   "Agora Ember",
		Tokens: tokens,
	}
}

func selectionFromManifest(manifest Manifest, source string) (Selection, error) {
	manifest = normalizeManifest(manifest)
	css, err := SafeTokenCSS(manifest.Tokens)
	if err != nil {
		return Selection{}, err
	}
	return Selection{
		Manifest: manifest,
		CSS:      css,
		Source:   source,
	}, nil
}

func normalizeManifest(manifest Manifest) Manifest {
	manifest = cloneManifest(manifest)
	tokens := DefaultTokens()
	for name, value := range manifest.Tokens {
		tokens[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	manifest.Tokens = tokens
	return manifest
}

func cloneManifest(manifest Manifest) Manifest {
	tokens := make(map[string]string, len(manifest.Tokens))
	for name, value := range manifest.Tokens {
		tokens[name] = value
	}
	return Manifest{
		ID:     manifest.ID,
		Name:   manifest.Name,
		Tokens: tokens,
	}
}

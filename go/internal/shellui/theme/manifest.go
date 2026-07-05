package theme

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Manifest struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Tokens map[string]string `json:"tokens"`
}

func DecodeManifest(reader io.Reader) (Manifest, error) {
	var manifest Manifest
	if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode theme manifest: %w", err)
	}
	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.Name = strings.TrimSpace(manifest.Name)
	if manifest.ID == "" {
		return Manifest{}, fmt.Errorf("theme manifest missing id")
	}
	if manifest.Name == "" {
		return Manifest{}, fmt.Errorf("theme manifest missing name")
	}
	if len(manifest.Tokens) == 0 {
		return Manifest{}, fmt.Errorf("theme manifest missing tokens")
	}
	for name, value := range manifest.Tokens {
		if err := ValidateToken(name, value); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

package widgets

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

type Manifest struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Entrypoint     string `json:"entrypoint"`
	BusTopicPrefix string `json:"busTopicPrefix"`
}

func DecodeManifest(reader io.Reader) (Manifest, error) {
	var manifest Manifest
	if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode widget manifest: %w", err)
	}
	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Entrypoint = strings.TrimSpace(manifest.Entrypoint)
	manifest.BusTopicPrefix = strings.TrimSpace(manifest.BusTopicPrefix)
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ValidateManifest(manifest Manifest) error {
	if manifest.ID == "" {
		return fmt.Errorf("widget manifest missing id")
	}
	if strings.ContainsAny(manifest.ID, "/\\.") {
		return fmt.Errorf("widget id %q must be a simple id", manifest.ID)
	}
	if manifest.Name == "" {
		return fmt.Errorf("widget %q missing name", manifest.ID)
	}
	if manifest.Entrypoint == "" {
		return fmt.Errorf("widget %q missing entrypoint", manifest.ID)
	}
	if path.Clean(manifest.Entrypoint) != manifest.Entrypoint || strings.HasPrefix(manifest.Entrypoint, "../") || strings.HasPrefix(manifest.Entrypoint, "/") {
		return fmt.Errorf("widget %q has unsafe entrypoint %q", manifest.ID, manifest.Entrypoint)
	}
	if manifest.BusTopicPrefix == "" {
		return fmt.Errorf("widget %q missing bus topic prefix", manifest.ID)
	}
	expectedPrefix := "widget." + manifest.ID
	if manifest.BusTopicPrefix != expectedPrefix {
		return fmt.Errorf("widget %q bus topic prefix = %q, want %q", manifest.ID, manifest.BusTopicPrefix, expectedPrefix)
	}
	return nil
}


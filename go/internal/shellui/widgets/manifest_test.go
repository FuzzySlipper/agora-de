package widgets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeManifestFixture(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "..", "..", "harness", "fixtures", "widgets", "clock-widget.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	manifest, err := DecodeManifest(file)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "clock" {
		t.Fatalf("widget id = %q, want clock", manifest.ID)
	}
	if manifest.BusTopicPrefix != "widget.clock" {
		t.Fatalf("topic prefix = %q, want widget.clock", manifest.BusTopicPrefix)
	}
}

func TestDecodeManifestRejectsUnsafeEntrypoint(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader(`{"id":"clock","name":"Clock","entrypoint":"../index.html","busTopicPrefix":"widget.clock"}`))
	if err == nil {
		t.Fatal("DecodeManifest accepted unsafe entrypoint")
	}
}

func TestDecodeManifestRejectsMismatchedTopicPrefix(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader(`{"id":"clock","name":"Clock","entrypoint":"index.html","busTopicPrefix":"widget.other"}`))
	if err == nil {
		t.Fatal("DecodeManifest accepted mismatched bus topic prefix")
	}
}

func TestRegistryAddGetList(t *testing.T) {
	registry := NewRegistry()
	manifest := Manifest{
		ID:             "clock",
		Name:           "Clock",
		Entrypoint:     "index.html",
		BusTopicPrefix: "widget.clock",
	}
	if err := registry.Add(manifest); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Get("clock"); !ok {
		t.Fatal("registry did not return clock")
	}
	if len(registry.List()) != 1 {
		t.Fatalf("registry list length = %d, want 1", len(registry.List()))
	}
}


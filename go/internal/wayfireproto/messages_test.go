package wayfireproto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodePluginEventFixture(t *testing.T) {
	file, err := os.Open(fixturePath(t, "plugin-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	types, err := DecodeStream(file)
	if err != nil {
		t.Fatal(err)
	}

	want := []MessageType{
		MessageTypeSurfaceEvent,
		MessageTypeSurfaceEvent,
		MessageTypeSurfaceEvent,
	}
	if len(types) != len(want) {
		t.Fatalf("decoded %d messages, want %d", len(types), len(want))
	}
	for index, got := range types {
		if got != want[index] {
			t.Fatalf("message %d type = %q, want %q", index, got, want[index])
		}
	}
}

func TestDecodeBridgeCommandFixture(t *testing.T) {
	file, err := os.Open(fixturePath(t, "bridge-commands.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	types, err := DecodeStream(file)
	if err != nil {
		t.Fatal(err)
	}

	want := []MessageType{
		MessageTypePolicyReplace,
		MessageTypePolicyUpsert,
		MessageTypePolicyRemove,
		MessageTypeInputContext,
		MessageTypeInputContext,
		MessageTypeCloseSurface,
		MessageTypeCloseSurfacesByUID,
	}
	if len(types) != len(want) {
		t.Fatalf("decoded %d messages, want %d", len(types), len(want))
	}
	for index, got := range types {
		if got != want[index] {
			t.Fatalf("message %d type = %q, want %q", index, got, want[index])
		}
	}
}

func TestDecodeLineRejectsUnknownType(t *testing.T) {
	_, err := DecodeLine([]byte(`{"type":"legacy_mode_workaround"}`))
	if err == nil {
		t.Fatal("DecodeLine accepted unknown message type")
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "compositor", "protocol-fixtures", "wayfire", name)
}


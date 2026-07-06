package wayfireproto

import (
	"os"
	"path/filepath"
	"strings"
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
		MessageTypePlaceSurface,
		MessageTypeSetSurfaceState,
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

func TestDecodeBackendLayoutStateFixture(t *testing.T) {
	file, err := os.Open(fixturePath(t, "layout-state-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	types, err := DecodeStream(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(types) != 1 || types[0] != MessageTypeLayoutState {
		t.Fatalf("decoded message types = %+v, want [%s]", types, MessageTypeLayoutState)
	}
}

func TestDecodeLayoutActionEventFixture(t *testing.T) {
	file, err := os.Open(fixturePath(t, "layout-action-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	types, err := DecodeStream(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(types) != 1 || types[0] != MessageTypePlaceResponse {
		t.Fatalf("decoded message types = %+v, want [%s]", types, MessageTypePlaceResponse)
	}
}

func TestDecodeSurfaceStateResponseFixture(t *testing.T) {
	messageType, err := DecodeLine([]byte(`{"type":"surface_state_response","request_id":"state-1","surface_id":"view-42","ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if messageType != MessageTypeSurfaceStateResponse {
		t.Fatalf("message type = %q, want %q", messageType, MessageTypeSurfaceStateResponse)
	}
}

func TestDecodeLineRejectsUnknownType(t *testing.T) {
	_, err := DecodeLine([]byte(`{"type":"legacy_mode_workaround"}`))
	if err == nil {
		t.Fatal("DecodeLine accepted unknown message type")
	}
}

func TestSurfaceEventKindsMatchGeneratedContract(t *testing.T) {
	contract, err := os.ReadFile(filepath.Join("..", "..", "..", "ts", "packages", "protocol", "src", "generated", "contracts.ts"))
	if err != nil {
		t.Fatal(err)
	}
	contractText := string(contract)
	eventKinds := []SurfaceEventKind{
		SurfaceEventMapped,
		SurfaceEventUnmapped,
		SurfaceEventFocused,
		SurfaceEventInputDenied,
	}
	for _, eventKind := range eventKinds {
		needle := "'" + string(eventKind) + "'"
		if !strings.Contains(contractText, needle) {
			t.Fatalf("generated contract is missing surface event kind %s", needle)
		}
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "compositor", "protocol-fixtures", "wayfire", name)
}

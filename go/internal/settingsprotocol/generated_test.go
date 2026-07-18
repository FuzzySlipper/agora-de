package settingsprotocol

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "harness", "fixtures", "settings", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func strictDecode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var value T
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode generated contract: %v", err)
	}
	return value
}

func assertRoundTrip[T any](t *testing.T, data []byte) T {
	t.Helper()
	value := strictDecode[T](t, data)
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode generated contract: %v", err)
	}
	decoded := strictDecode[T](t, encoded)
	if !reflect.DeepEqual(value, decoded) {
		t.Fatalf("round trip changed value: %#v != %#v", value, decoded)
	}
	return value
}

func TestCatalogFixtureRoundTripsAcrossGeneratedContract(t *testing.T) {
	catalog := assertRoundTrip[SettingsCatalogResponse](t, fixture(t, "catalog-v1.json"))
	if catalog.SchemaVersion != SettingsSchemaVersion || len(catalog.Modules) != 2 {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	if got := catalog.Modules[1].Availability.State; got != SettingsAvailabilityUnsupported {
		t.Fatalf("unknown module availability = %q, want unsupported", got)
	}
}

func TestDiagnosticsFixturesRoundTripWithoutGenericPayload(t *testing.T) {
	state := assertRoundTrip[DiagnosticsSettingsState](t, fixture(t, "diagnostics-state-v1.json"))
	if state.ModuleID != DiagnosticsModuleID || state.Revision != 7 {
		t.Fatalf("unexpected diagnostics state: %+v", state)
	}
	request := assertRoundTrip[DiagnosticsApplyRequest](t, fixture(t, "diagnostics-apply-request-v1.json"))
	if request.Draft.DiagnosticOverlayEnabled {
		t.Fatalf("fixture should disable the overlay: %+v", request)
	}
}

func TestStrictDecoderRejectsUnknownAndRemovedFields(t *testing.T) {
	unknown := []byte(`{"contractVersion":1,"baseRevision":7,"draft":{"diagnosticOverlayEnabled":false,"command":"nope"}}`)
	var request DiagnosticsApplyRequest
	decoder := json.NewDecoder(bytes.NewReader(unknown))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err == nil {
		t.Fatal("unknown command field was accepted")
	}

	removed := []byte(`{"contractVersion":1,"draft":{"diagnosticOverlayEnabled":false}}`)
	request = DiagnosticsApplyRequest{}
	if err := json.Unmarshal(removed, &request); err == nil {
		t.Fatal("removed baseRevision field was accepted")
	}
}

package settingsdiagnostics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agora-de.local/go/internal/settingsprotocol"
)

func fakeSystemctl(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	command := filepath.Join(dir, "systemctl")
	logPath := filepath.Join(dir, "calls.log")
	enabledPath := filepath.Join(dir, "enabled")
	activePath := filepath.Join(dir, "active")
	script := `#!/usr/bin/env sh
set -eu
printf '%s\n' "$*" >> "$CALL_LOG"
case "$*" in
  "--user is-enabled agora-de-shell-overlay.service")
    if [ -f "$ENABLED_PATH" ]; then printf '%s\n' enabled; else printf '%s\n' disabled; fi
    ;;
  "--user is-active agora-de-shell-overlay.service")
    if [ -f "$ACTIVE_PATH" ]; then printf '%s\n' active; else printf '%s\n' inactive; fi
    ;;
  "--user enable --now agora-de-shell-overlay.service")
    if [ "${FAIL_APPLY:-}" = "1" ]; then printf '%s\n' 'enable failed' >&2; exit 1; fi
    : > "$ENABLED_PATH"
    : > "$ACTIVE_PATH"
    ;;
  "--user disable --now agora-de-shell-overlay.service")
    if [ "${FAIL_APPLY:-}" = "1" ]; then printf '%s\n' 'disable failed' >&2; exit 1; fi
    rm -f "$ENABLED_PATH" "$ACTIVE_PATH"
    ;;
  *)
    printf 'unexpected systemctl command: %s\n' "$*" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", logPath)
	t.Setenv("ENABLED_PATH", enabledPath)
	t.Setenv("ACTIVE_PATH", activePath)
	return command
}

func request(t *testing.T, module *Module, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(method, path, strings.NewReader(body))
	module.Handler().ServeHTTP(recorder, httpRequest)
	return recorder
}

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return value
}

func TestDiagnosticsLifecycleUsesGeneratedContracts(t *testing.T) {
	module := New(Config{SystemctlPath: fakeSystemctl(t)})
	initialResponse := request(t, module, http.MethodGet, "/state", "")
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial status = %d: %s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decode[settingsprotocol.DiagnosticsSettingsState](t, initialResponse)
	if initial.Revision != 1 || initial.Active.DiagnosticOverlayEnabled || initial.ModuleID != settingsprotocol.DiagnosticsModuleID {
		t.Fatalf("unexpected initial state: %+v", initial)
	}

	validateResponse := request(t, module, http.MethodPost, "/validate", `{"contractVersion":1,"baseRevision":1,"draft":{"diagnosticOverlayEnabled":true}}`)
	validation := decode[settingsprotocol.SettingsValidationResponse](t, validateResponse)
	if validateResponse.Code != http.StatusOK || !validation.Valid {
		t.Fatalf("validation = %d %+v", validateResponse.Code, validation)
	}

	applyResponse := request(t, module, http.MethodPost, "/apply", `{"contractVersion":1,"baseRevision":1,"draft":{"diagnosticOverlayEnabled":true}}`)
	if applyResponse.Code != http.StatusOK {
		t.Fatalf("apply status = %d: %s", applyResponse.Code, applyResponse.Body.String())
	}
	applied := decode[settingsprotocol.DiagnosticsApplyResponse](t, applyResponse)
	if !applied.State.Active.DiagnosticOverlayEnabled || !applied.State.Service.Active || applied.State.Revision != 2 {
		t.Fatalf("unexpected applied state: %+v", applied)
	}

	staleResponse := request(t, module, http.MethodPost, "/apply", `{"contractVersion":1,"baseRevision":1,"draft":{"diagnosticOverlayEnabled":false}}`)
	stale := decode[settingsprotocol.SettingsError](t, staleResponse)
	if staleResponse.Code != http.StatusConflict || stale.Code != settingsprotocol.SettingsErrorStaleRevision {
		t.Fatalf("stale response = %d %+v", staleResponse.Code, stale)
	}

	defaultsResponse := request(t, module, http.MethodPost, "/defaults", `{"contractVersion":1,"baseRevision":2}`)
	defaults := decode[settingsprotocol.DiagnosticsSettings](t, defaultsResponse)
	if defaultsResponse.Code != http.StatusOK || defaults.DiagnosticOverlayEnabled {
		t.Fatalf("defaults = %d %+v", defaultsResponse.Code, defaults)
	}
}

func TestDiagnosticsValidationAndStrictDecodeFailures(t *testing.T) {
	module := New(Config{SystemctlPath: fakeSystemctl(t)})
	invalidVersion := request(t, module, http.MethodPost, "/validate", `{"contractVersion":9,"baseRevision":1,"draft":{"diagnosticOverlayEnabled":true}}`)
	validation := decode[settingsprotocol.SettingsValidationResponse](t, invalidVersion)
	if invalidVersion.Code != http.StatusOK || validation.Valid || len(validation.Issues) != 1 {
		t.Fatalf("invalid version response = %d %+v", invalidVersion.Code, validation)
	}

	unknown := request(t, module, http.MethodPost, "/apply", `{"contractVersion":1,"baseRevision":1,"draft":{"diagnosticOverlayEnabled":true,"command":"nope"}}`)
	settingsError := decode[settingsprotocol.SettingsError](t, unknown)
	if unknown.Code != http.StatusBadRequest || settingsError.Code != settingsprotocol.SettingsErrorInvalidRequest {
		t.Fatalf("unknown field response = %d %+v", unknown.Code, settingsError)
	}
}

func TestDiagnosticsUnavailableAndBackendFailure(t *testing.T) {
	unavailable := New(Config{SystemctlPath: filepath.Join(t.TempDir(), "missing-systemctl")})
	unavailableResponse := request(t, unavailable, http.MethodGet, "/state", "")
	unavailableError := decode[settingsprotocol.SettingsError](t, unavailableResponse)
	if unavailableResponse.Code != http.StatusServiceUnavailable || unavailableError.Code != settingsprotocol.SettingsErrorUnavailable {
		t.Fatalf("unavailable response = %d %+v", unavailableResponse.Code, unavailableError)
	}

	module := New(Config{SystemctlPath: fakeSystemctl(t)})
	t.Setenv("FAIL_APPLY", "1")
	failureResponse := request(t, module, http.MethodPost, "/apply", `{"contractVersion":1,"baseRevision":1,"draft":{"diagnosticOverlayEnabled":true}}`)
	failure := decode[settingsprotocol.SettingsError](t, failureResponse)
	if failureResponse.Code != http.StatusServiceUnavailable || failure.Code != settingsprotocol.SettingsErrorApplyFailed {
		t.Fatalf("backend failure response = %d %+v", failureResponse.Code, failure)
	}
}

func TestSupportBundleIsAllowlistedAndBounded(t *testing.T) {
	module := New(Config{SystemctlPath: fakeSystemctl(t)})
	state := module.snapshot(t.Context())
	data, err := json.Marshal(state.SupportBundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.SupportBundle.Components) != 4 || len(data) > 16*1024 {
		t.Fatalf("bundle size/components = %d/%d", len(data), len(state.SupportBundle.Components))
	}
	text := string(data)
	for _, forbidden := range []string{"CALL_LOG", os.Getenv("HOME"), "windowTitle", "clipboard", "journal"} {
		if forbidden != "" && strings.Contains(text, forbidden) {
			t.Fatalf("bundle leaked %q: %s", forbidden, text)
		}
	}
}

package settingsdisplays

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agora-de.local/go/internal/settingsprotocol"
)

const fixtureState = `{"moduleId":"displays","contractVersion":1,"revision":7,"active":{"serial":44,"heads":[{"id":"HDMI-A-1","identity":{"name":"HDMI-A-1","description":"Fixture display","make":"Agora","model":"Panel","serialNumber":"serial-1","physicalWidthMm":600,"physicalHeightMm":340},"connected":true,"enabled":true,"modes":[{"id":"2560x1440@60000","width":2560,"height":1440,"refreshMillihz":60000,"preferred":true}],"currentModeId":"2560x1440@60000","x":0,"y":0,"scaleMilli":1250,"transform":"rotate_90","adaptiveSync":false}]},"defaults":{"serial":44,"heads":[]},"capabilities":{"outputManagement":true,"protocolVersion":4,"testConfiguration":true,"applyConfiguration":true,"adaptiveSync":true},"reconciliation":{"state":"not_needed","detail":"fixture","matchedHeads":[],"unmatchedProfileHeads":[]},"availability":{"state":"available"}}`

func TestModuleLoadsGeneratedDisplayState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}
	path := fixtureAuthority(t, fixtureState)
	module := New(Config{AuthorityPath: path})
	if got := module.Availability(context.Background()); got.State != settingsprotocol.SettingsAvailabilityAvailable {
		t.Fatalf("availability = %+v", got)
	}
	recorder := httptest.NewRecorder()
	module.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/state", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"scaleMilli":1250`) || !strings.Contains(recorder.Body.String(), `"transform":"rotate_90"`) {
		t.Fatalf("response = status %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestModuleReportsMissingAuthorityAsUnavailable(t *testing.T) {
	module := New(Config{AuthorityPath: filepath.Join(t.TempDir(), "missing-authority")})
	availability := module.Availability(context.Background())
	if availability.State != settingsprotocol.SettingsAvailabilityUnavailable || availability.Reason == "" {
		t.Fatalf("availability = %+v", availability)
	}
}

func TestModuleForwardsTypedMutationsAndRejectsUnknownFields(t *testing.T) {
	validation := `{"valid":true,"issues":[]}`
	apply := `{"state":` + fixtureState + `,"outcome":{"kind":"pending_confirmation"}}`
	path := fixtureCommandAuthority(t, map[string]string{
		"snapshot": fixtureState,
		"validate": validation,
		"apply":    apply,
		"keep":     strings.Replace(apply, "pending_confirmation", "kept", 1),
	})
	module := New(Config{AuthorityPath: path, StateDir: t.TempDir()})

	validateBody := `{"contractVersion":1,"baseRevision":7,"draft":{"serial":44,"heads":[]}}`
	for _, test := range []struct {
		path string
		body string
		want string
	}{
		{"/validate", validateBody, `"valid":true`},
		{"/apply", `{"contractVersion":1,"baseRevision":7,"draft":{"serial":44,"heads":[]},"confirmationTimeoutMillis":15000}`, `"pending_confirmation"`},
		{"/keep", `{"contractVersion":1,"transactionId":"tx-1"}`, `"kind":"kept"`},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
		module.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.want) {
			t.Fatalf("%s response = status %d body %s", test.path, recorder.Code, recorder.Body.String())
		}
	}

	recorder := httptest.NewRecorder()
	module.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(`{"contractVersion":1,"baseRevision":7,"draft":{"serial":44,"heads":[]},"confirmationTimeoutMillis":15000,"unknown":true}`)))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "unknown field") {
		t.Fatalf("strict response = status %d body %s", recorder.Code, recorder.Body.String())
	}
}

func fixtureAuthority(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "display-authority-fixture")
	script := "#!/usr/bin/env sh\nprintf '%s\\n' '" + body + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fixtureCommandAuthority(t *testing.T, responses map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "display-authority-fixture")
	script := "#!/usr/bin/env sh\ncase \"$1\" in\n"
	for command, response := range responses {
		script += fmt.Sprintf("%s) printf '%%s\\n' '%s' ;;\n", command, response)
	}
	script += "*) exit 2 ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

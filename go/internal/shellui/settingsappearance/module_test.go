package settingsappearance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agora-de.local/go/internal/settingsprotocol"
	"agora-de.local/go/internal/shellui/theme"
)

func TestAppearanceCatalogValidationAndAtomicPersistence(t *testing.T) {
	module := New(Config{StateDir: t.TempDir(), ActiveThemeID: theme.DefaultThemeID})
	state := module.snapshot()
	if len(state.Themes) < 2 || state.Active.ThemeID != theme.DefaultThemeID {
		t.Fatalf("state = %+v", state)
	}
	request := settingsprotocol.AppearanceApplyRequest{ContractVersion: settingsprotocol.AppearanceContractVersion, BaseRevision: state.Revision, Draft: settingsprotocol.AppearanceSettings{ThemeID: theme.EmberThemeID}}
	body, _ := json.Marshal(request)
	recorder := httptest.NewRecorder()
	module.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/apply", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("apply status=%d body=%s", recorder.Code, recorder.Body)
	}
	if got := module.snapshot(); got.Active.ThemeID != theme.EmberThemeID || !got.RestartRequired {
		t.Fatalf("applied state=%+v", got)
	}
}

func TestAppearanceRejectsUnknownAndStaleThemes(t *testing.T) {
	module := New(Config{StateDir: t.TempDir()})
	state := module.snapshot()
	for _, request := range []settingsprotocol.AppearanceApplyRequest{
		{ContractVersion: 1, BaseRevision: state.Revision, Draft: settingsprotocol.AppearanceSettings{ThemeID: "unknown"}},
		{ContractVersion: 1, BaseRevision: state.Revision + 1, Draft: settingsprotocol.AppearanceSettings{ThemeID: theme.EmberThemeID}},
	} {
		body, _ := json.Marshal(request)
		recorder := httptest.NewRecorder()
		module.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/apply", bytes.NewReader(body)))
		if recorder.Code < 400 {
			t.Fatalf("request %+v unexpectedly passed", request)
		}
	}
}

package settingswindowmanagement

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agora-de.local/go/internal/settingsprotocol"
)

const layoutFixture = `{"layout":{"mode":"columns","revision":7,"settings":{"rule":"master_stack","mode":"columns","gaps":{"outer_horizontal":4,"outer_vertical":6,"inner_horizontal":8,"inner_vertical":10},"master_count":2,"master_ratio":0.6,"smart_gaps":false},"workspaces":[{"id":"workspace-1","name":"workspace 1","output_id":"HDMI-A-1","active":true,"surface_order":["surface-1"]}]}}`

func TestLifecycleAndValidation(t *testing.T) {
	path := fixtureCLI(t)
	module := New(Config{CompositorctlPath: path})
	if module.Availability(context.Background()).State != settingsprotocol.SettingsAvailabilityAvailable {
		t.Fatal("module should be available")
	}
	recorder := httptest.NewRecorder()
	module.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/state", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"masterRatio":0.6`) || !strings.Contains(recorder.Body.String(), `"surfaceCount":1`) {
		t.Fatalf("state %d %s", recorder.Code, recorder.Body.String())
	}
	invalidBody := `{"contractVersion":1,"baseRevision":7,"draft":{"mode":"columns","rule":"master_stack","gaps":{"outerHorizontal":0,"outerVertical":0,"innerHorizontal":0,"innerVertical":0},"masterCount":0,"masterRatio":0.6,"smartGaps":true}}`
	recorder = httptest.NewRecorder()
	module.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(invalidBody)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"valid":false`) {
		t.Fatalf("validation %d %s", recorder.Code, recorder.Body.String())
	}
	stale := strings.Replace(invalidBody, `"baseRevision":7`, `"baseRevision":6`, 1)
	recorder = httptest.NewRecorder()
	module.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(stale)))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("stale status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func fixtureCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "compositorctl")
	script := "#!/usr/bin/env sh\nif [ \"$1 $2\" = \"layout get\" ]; then printf '%s\\n' '" + layoutFixture + "'; exit 0; fi\nif [ \"$1 $2\" = \"layout set-settings\" ]; then printf '%s\\n' '{\"status\":\"accepted\"}'; exit 0; fi\nexit 2\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

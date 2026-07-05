package layoutroute

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"agora-de.local/go/internal/shellui/surfaces"
)

func TestLayoutFallsBackToSurfaceProjectionWhenBackendUnsupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture requires a POSIX shell")
	}
	compositorctl := filepath.Join(t.TempDir(), "agora-de-compositorctl")
	if err := os.WriteFile(compositorctl, []byte("#!/usr/bin/env sh\necho 'error: server[backend_unsupported]: unsupported compositor method \"get_layout\"' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	provider := func(*http.Request) ([]surfaces.SurfaceView, error) {
		return []surfaces.SurfaceView{
			{
				ID:          "view-1",
				Label:       "1",
				AppID:       "Alacritty",
				Title:       "Terminal",
				Mapped:      true,
				Focused:     true,
				Visible:     true,
				WorkspaceID: "workspace-1",
				ZoneID:      "primary",
				LayoutMode:  "zones",
				LayoutRole:  "tiled",
				Geometry:    &surfaces.GeometryView{X: 24, Y: 48, Width: 800, Height: 600},
				SurfaceKind: "xdg_toplevel",
				OutputID:    "HDMI-A-1",
				OwnerUID:    1001,
			},
			{
				ID:          "layer-1",
				AppID:       "io.agorade.ShellOverlay",
				Mapped:      true,
				Visible:     true,
				SurfaceKind: "layer_shell",
			},
		}, nil
	}
	handler := New(Config{
		CompositorctlPath: compositorctl,
		UseCompositorctl:  true,
		SurfaceProvider:   provider,
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, LayoutPath, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var response layoutResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Layout.Mode != "freeform" || len(response.Layout.Surfaces) != 1 {
		t.Fatalf("unexpected layout response: %+v", response)
	}
	surface := response.Layout.Surfaces[0]
	if surface.SurfaceID != "view-1" || surface.Label != "1" || !surface.Focused || surface.Geometry == nil {
		t.Fatalf("unexpected fallback layout surface: %+v", surface)
	}
	if surface.Geometry.X != 24 || surface.Geometry.Width != 800 {
		t.Fatalf("unexpected fallback geometry: %+v", surface.Geometry)
	}
}

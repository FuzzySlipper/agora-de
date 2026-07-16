package layoutroute

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
				ID:          "view-dialog",
				Label:       "D",
				AppID:       "Alacritty",
				Title:       "Open File",
				Role:        "file-chooser-dialog",
				Mapped:      true,
				Visible:     true,
				WorkspaceID: "workspace-1",
				ZoneID:      "transient",
				LayoutMode:  "freeform",
				LayoutRole:  "transient",
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
	if response.Layout.Mode != "freeform" || len(response.Layout.Surfaces) != 2 {
		t.Fatalf("unexpected layout response: %+v", response)
	}
	surface := response.Layout.Surfaces[0]
	if surface.SurfaceID != "view-1" || surface.Label != "1" || !surface.Focused || surface.Geometry == nil {
		t.Fatalf("unexpected fallback layout surface: %+v", surface)
	}
	if surface.Geometry.X != 24 || surface.Geometry.Width != 800 {
		t.Fatalf("unexpected fallback geometry: %+v", surface.Geometry)
	}
	if surface.PolicyClass != "work" || surface.PolicyReason == "" {
		t.Fatalf("fallback work surface policy = %+v", surface)
	}
	dialog := response.Layout.Surfaces[1]
	if dialog.SurfaceID != "view-dialog" || dialog.Participation != "transient" || dialog.PolicyClass != "no_parent" || dialog.PolicyReason == "" {
		t.Fatalf("fallback dialog policy = %+v", dialog)
	}
}

func TestLayoutZoneActionsForwardPlannerGeometry(t *testing.T) {
	args, err := actionArgs(actionRequest{
		Action:      "assignZone",
		SurfaceID:   " view-live ",
		WorkspaceID: " workspace-1 ",
		ZoneID:      " stack ",
		Geometry:    &surfaces.GeometryView{X: 320, Y: 64, Width: 960, Height: 720},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"surface", "assign-zone",
		"--surface", "view-live",
		"--zone", "stack",
		"--workspace", "workspace-1",
		"--x", "320",
		"--y", "64",
		"--width", "960",
		"--height", "720",
		"--timeout-ms", "2000",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestLayoutPromoteActionForwardsSurface(t *testing.T) {
	args, err := actionArgs(actionRequest{
		Action:    "promote",
		SurfaceID: " view-live ",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"surface", "promote",
		"--surface", "view-live",
		"--timeout-ms", "2000",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestLayoutMoveActionForwardsDirection(t *testing.T) {
	args, err := actionArgs(actionRequest{
		Action:    "move",
		SurfaceID: "view-live",
		Direction: "left",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"surface", "move",
		"--surface", "view-live",
		"--direction", "left",
		"--timeout-ms", "2000",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestLayoutMoveActionRejectsInvalidDirection(t *testing.T) {
	_, err := actionArgs(actionRequest{Action: "move", SurfaceID: "view-live", Direction: "sideways"})
	if err == nil || !strings.Contains(err.Error(), "direction must be") {
		t.Fatalf("err = %v, want direction validation error", err)
	}
}

func TestLayoutSwapMasterActionForwardsSurface(t *testing.T) {
	args, err := actionArgs(actionRequest{
		Action:    "swapMaster",
		SurfaceID: "view-live",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"surface", "swap-master",
		"--surface", "view-live",
		"--timeout-ms", "2000",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestLayoutZoneActionsRejectInvalidPlannerGeometry(t *testing.T) {
	_, err := actionArgs(actionRequest{
		Action:    "tile",
		SurfaceID: "view-live",
		ZoneID:    "master",
		Geometry:  &surfaces.GeometryView{Width: 0, Height: 720},
	})
	if err == nil || !strings.Contains(err.Error(), "geometry width and height must be positive") {
		t.Fatalf("err = %v, want positive geometry error", err)
	}
}

func TestLayoutSetFloatingActionCanDisableFloating(t *testing.T) {
	enabled := false
	args, err := actionArgs(actionRequest{
		Action:    "setFloating",
		SurfaceID: "view-live",
		Floating:  &enabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"surface", "set-floating",
		"--surface", "view-live",
		"--enabled=false",
		"--timeout-ms", "2000",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestLayoutSettingsActionForwardsBackendOwnedSettings(t *testing.T) {
	args, err := actionArgs(actionRequest{
		Action: "setSettings",
		Settings: &layoutSettings{
			Rule:        "dwindle",
			Mode:        "columns",
			MasterCount: 2,
			MasterRatio: 0.6,
			SmartGaps:   false,
			Gaps: layoutGaps{
				OuterHorizontal: 4,
				OuterVertical:   6,
				InnerHorizontal: 8,
				InnerVertical:   10,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"layout", "set-settings",
		"--rule", "dwindle",
		"--mode", "columns",
		"--outer-horizontal", "4",
		"--outer-vertical", "6",
		"--inner-horizontal", "8",
		"--inner-vertical", "10",
		"--master-count", "2",
		"--master-ratio", "0.60",
		"--smart-gaps=false",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

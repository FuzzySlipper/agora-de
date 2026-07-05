package compositorbridge

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListSurfacesTracksMappedFocusedAndUnmappedEvents(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-1",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "App",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 800, Height: 600},
			OutputID:    "HDMI-A-1",
		},
		Client: ClientIdentity{PID: 12, UID: 1001, GID: 1002},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventContentCommit,
		Surface: CompositorSurface{ID: "view-1", Visible: &visible},
		Client:  ClientIdentity{PID: 12, UID: 1001, GID: 1002},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventFocused,
		Surface: CompositorSurface{ID: "view-1", Visible: &visible},
		Client:  ClientIdentity{PID: 12, UID: 1001, GID: 1002},
	})

	surfaces := bridge.ListSurfaces()
	if len(surfaces) != 1 {
		t.Fatalf("surfaces = %d, want 1", len(surfaces))
	}
	if !surfaces[0].Focused || surfaces[0].ContentCommitCount != 1 || surfaces[0].OutputID != "HDMI-A-1" {
		t.Fatalf("surface readback not retained: %+v", surfaces[0])
	}

	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventUnmapped,
		Surface: CompositorSurface{ID: "view-1"},
	})
	if got := bridge.ListSurfaces(); len(got) != 0 {
		t.Fatalf("surfaces after unmap = %+v", got)
	}
}

func TestCloseSurfaceQueuesPluginMessage(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	defer pluginServer.Close()
	bridge.installPlugin(&pluginSession{conn: pluginServer, enc: json.NewEncoder(pluginServer)})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMapped,
		Surface: CompositorSurface{ID: "view-close", SurfaceKind: SurfaceKindXDG, Visible: &visible},
	})

	done := make(chan map[string]any, 1)
	go func() {
		var message map[string]any
		_ = json.NewDecoder(pluginClient).Decode(&message)
		done <- message
	}()

	response, err := bridge.CloseSurface(CloseSurfaceRequest{SurfaceID: "view-close"})
	if err != nil {
		t.Fatalf("CloseSurface: %v", err)
	}
	if response.Decision != DecisionAccepted || !response.Queued || response.ClosedSurfaceID != "view-close" {
		t.Fatalf("response = %+v", response)
	}
	select {
	case message := <-done:
		if message["type"] != PluginCloseSurface || message["surface_id"] != "view-close" {
			t.Fatalf("plugin message = %+v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for plugin close message")
	}
}

func TestListOutputsIncludesPhysicalSurfaceReadback(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "layer-panel",
			SurfaceKind: SurfaceKindLayer,
			Visible:     &visible,
			OutputID:    "HDMI-A-1",
			Geometry:    &SurfaceGeometry{Width: 2560, Height: 96},
		},
	})
	outputs := bridge.ListOutputs()
	if len(outputs) != 1 {
		t.Fatalf("outputs = %+v", outputs)
	}
	if outputs[0].Name != "HDMI-A-1" || outputs[0].Mode != "physical_surface_readback" || outputs[0].Surfaces[0] != "layer-panel" {
		t.Fatalf("output = %+v", outputs[0])
	}
}

func TestCaptureOutputUsesPhysicalGrimBackend(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	fakeGrim := filepath.Join(dir, "grim")
	script := "#!/bin/sh\nout=\"\"\nwhile [ \"$#\" -gt 0 ]; do out=\"$1\"; shift; done\ncp \"$FAKE_CAPTURE_PNG\" \"$out\"\n"
	if err := os.WriteFile(fakeGrim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGORA_OUTPUT_CAPTURE_GRIM", fakeGrim)
	t.Setenv("AGORA_OUTPUT_CAPTURE_USER", "")
	t.Setenv("FAKE_CAPTURE_PNG", source)
	t.Setenv("AGORA_CAPTURE_ROOT", filepath.Join(dir, "captures"))
	t.Setenv("AGORA_ARTIFACT_ROOT", filepath.Join(dir, "artifacts"))

	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMapped,
		Surface: CompositorSurface{ID: "layer-panel", SurfaceKind: SurfaceKindLayer, Visible: &visible, OutputID: "HDMI-A-1", Geometry: &SurfaceGeometry{Width: 2, Height: 1}},
	})

	response, err := bridge.CaptureOutput(CaptureOutputRequest{Name: "HDMI-A-1", Export: true, SessionID: "session-1"})
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}
	if len(response.Captures) != 1 {
		t.Fatalf("captures = %+v", response.Captures)
	}
	capture := response.Captures[0]
	if capture.SurfaceID != "output:HDMI-A-1" || capture.Width != 2 || capture.Height != 1 || capture.Artifact == nil {
		t.Fatalf("capture = %+v", capture)
	}
	if capture.VisualInspection == nil || capture.VisualInspection.Status != "visible" {
		t.Fatalf("inspection = %+v", capture.VisualInspection)
	}
}

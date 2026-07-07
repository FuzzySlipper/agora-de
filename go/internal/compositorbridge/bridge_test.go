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

func TestSurfaceStateActionsRoundTripThroughPlugin(t *testing.T) {
	for _, tc := range []struct {
		name       string
		action     string
		stateField string
		call       func(*Bridge, SurfaceLayoutActionRequest) (LayoutActionResponse, error)
	}{
		{name: "maximize", action: "surface.maximize", stateField: "maximized", call: (*Bridge).MaximizeSurface},
		{name: "minimize", action: "surface.minimize", stateField: "minimized", call: (*Bridge).MinimizeSurface},
		{name: "fullscreen", action: "surface.fullscreen", stateField: "fullscreen", call: (*Bridge).FullscreenSurface},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bridge := New(Config{})
			pluginClient, pluginServer := net.Pipe()
			defer pluginClient.Close()
			defer pluginServer.Close()
			bridge.installPlugin(&pluginSession{conn: pluginServer, enc: json.NewEncoder(pluginServer)})
			visible := true
			bridge.handleSurfaceEvent(pluginEvent{
				Type:    PluginSurfaceEvent,
				Event:   EventMapped,
				Surface: CompositorSurface{ID: "view-state", SurfaceKind: SurfaceKindXDG, Visible: &visible},
			})

			done := make(chan struct {
				response LayoutActionResponse
				err      error
			}, 1)
			go func() {
				response, err := tc.call(bridge, SurfaceLayoutActionRequest{SurfaceID: "view-state", WaitTimeoutMs: 5000})
				done <- struct {
					response LayoutActionResponse
					err      error
				}{response: response, err: err}
			}()

			var command map[string]any
			if err := json.NewDecoder(pluginClient).Decode(&command); err != nil {
				t.Fatalf("decode state command: %v", err)
			}
			requestID, ok := command["request_id"].(string)
			if !ok || requestID == "" {
				t.Fatalf("state command missing request_id: %+v", command)
			}
			if command["type"] != PluginSetSurfaceState || command["surface_id"] != "view-state" || command[tc.stateField] != true {
				t.Fatalf("state command = %+v", command)
			}
			for _, other := range []string{"fullscreen", "maximized", "minimized"} {
				if other != tc.stateField && command[other] != nil {
					t.Fatalf("state command set unrelated field %q: %+v", other, command)
				}
			}
			bridge.handlePluginEvent(pluginEvent{Type: PluginSurfaceStateResponse, RequestID: requestID, SurfaceID: "view-state", OK: true})

			select {
			case result := <-done:
				if result.err != nil {
					t.Fatalf("%s: %v", tc.name, result.err)
				}
				if result.response.Action != tc.action || result.response.SurfaceID != "view-state" || result.response.Decision != DecisionAccepted || result.response.Surface == nil {
					t.Fatalf("response = %+v", result.response)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for state response")
			}
		})
	}
}

func TestSurfaceStateActionFailsFastWhenPluginDisconnects(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	defer pluginServer.Close()
	session := &pluginSession{conn: pluginServer, enc: json.NewEncoder(pluginServer)}
	bridge.installPlugin(session)
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMapped,
		Surface: CompositorSurface{ID: "view-state", SurfaceKind: SurfaceKindXDG, Visible: &visible},
	})

	done := make(chan error, 1)
	go func() {
		_, err := bridge.FullscreenSurface(SurfaceLayoutActionRequest{SurfaceID: "view-state", WaitTimeoutMs: 5000})
		done <- err
	}()

	var command map[string]any
	if err := json.NewDecoder(pluginClient).Decode(&command); err != nil {
		t.Fatalf("decode state command: %v", err)
	}
	if command["type"] != PluginSetSurfaceState || command["surface_id"] != "view-state" || command["fullscreen"] != true {
		t.Fatalf("state command = %+v", command)
	}
	bridge.clearPlugin(session)

	select {
	case err := <-done:
		if class, _ := classifyError(err); class != ErrorCompositorUnavailable {
			t.Fatalf("state error = %v class=%s, want %s", err, class, ErrorCompositorUnavailable)
		}
	case <-time.After(time.Second):
		t.Fatal("state request waited after plugin disconnect")
	}
}

func TestMinimizeRestoreAllowsMinimizedInvisibleSurface(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	defer pluginServer.Close()
	bridge.installPlugin(&pluginSession{conn: pluginServer, enc: json.NewEncoder(pluginServer)})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMapped,
		Surface: CompositorSurface{ID: "view-state", SurfaceKind: SurfaceKindXDG, Visible: &visible},
	})
	minimizedVisible := false
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMinimized,
		Surface: CompositorSurface{ID: "view-state", SurfaceKind: SurfaceKindXDG, Visible: &minimizedVisible},
	})

	enabled := false
	done := make(chan error, 1)
	go func() {
		_, err := bridge.MinimizeSurface(SurfaceLayoutActionRequest{SurfaceID: "view-state", Enabled: &enabled, WaitTimeoutMs: 5000})
		done <- err
	}()

	var command map[string]any
	if err := json.NewDecoder(pluginClient).Decode(&command); err != nil {
		t.Fatalf("decode restore command: %v", err)
	}
	requestID, ok := command["request_id"].(string)
	if !ok || requestID == "" {
		t.Fatalf("state command missing request_id: %+v", command)
	}
	if command["type"] != PluginSetSurfaceState || command["surface_id"] != "view-state" || command["minimized"] != false {
		t.Fatalf("restore command = %+v", command)
	}
	bridge.handlePluginEvent(pluginEvent{Type: PluginSurfaceStateResponse, RequestID: requestID, SurfaceID: "view-state", OK: true})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("restore minimize: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for restore response")
	}
}

func TestPlaceSurfaceFailsFastWhenPluginDisconnects(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	defer pluginServer.Close()
	session := &pluginSession{conn: pluginServer, enc: json.NewEncoder(pluginServer)}
	bridge.installPlugin(session)
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMapped,
		Surface: CompositorSurface{ID: "view-place", SurfaceKind: SurfaceKindXDG, Visible: &visible},
	})

	done := make(chan error, 1)
	go func() {
		_, err := bridge.placeSurfaceChecked(
			SurfaceLayoutActionRequest{SurfaceID: "view-place", WaitTimeoutMs: 5000},
			"test.place",
			SurfaceGeometry{X: 0, Y: 0, Width: 100, Height: 100},
			zoneMaster,
			SurfaceLayoutRoleTiled,
			nil,
			true,
		)
		done <- err
	}()

	var command map[string]any
	if err := json.NewDecoder(pluginClient).Decode(&command); err != nil {
		t.Fatalf("decode place command: %v", err)
	}
	if command["type"] != PluginPlaceSurface || command["surface_id"] != "view-place" {
		t.Fatalf("place command = %+v", command)
	}
	bridge.clearPlugin(session)

	select {
	case err := <-done:
		if class, _ := classifyError(err); class != ErrorCompositorUnavailable {
			t.Fatalf("place error = %v class=%s, want %s", err, class, ErrorCompositorUnavailable)
		}
	case <-time.After(time.Second):
		t.Fatal("place request waited after plugin disconnect")
	}
}

func TestFocusSurfaceFailsFastWhenPluginDisconnects(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	defer pluginServer.Close()
	session := &pluginSession{conn: pluginServer, enc: json.NewEncoder(pluginServer)}
	bridge.installPlugin(session)
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMapped,
		Surface: CompositorSurface{ID: "view-focus", SurfaceKind: SurfaceKindXDG, Visible: &visible},
	})

	done := make(chan error, 1)
	go func() {
		_, err := bridge.FocusSurface(FocusSurfaceRequest{SurfaceID: "view-focus", WaitTimeoutMs: 5000})
		done <- err
	}()

	var command map[string]any
	if err := json.NewDecoder(pluginClient).Decode(&command); err != nil {
		t.Fatalf("decode focus command: %v", err)
	}
	if command["type"] != PluginFocusSurface || command["surface_id"] != "view-focus" {
		t.Fatalf("focus command = %+v", command)
	}
	bridge.clearPlugin(session)

	select {
	case err := <-done:
		if class, _ := classifyError(err); class != ErrorCompositorUnavailable {
			t.Fatalf("focus error = %v class=%s, want %s", err, class, ErrorCompositorUnavailable)
		}
	case <-time.After(time.Second):
		t.Fatal("focus request waited after plugin disconnect")
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

func TestListSurfacesPrunesOldLayerShellSurfaceWithDeadClient(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "layer-stale",
			SurfaceKind: SurfaceKindLayer,
			AppID:       "io.agorade.ShellPanel",
			Visible:     &visible,
			OutputID:    "HDMI-A-1",
			Geometry:    &SurfaceGeometry{Width: 2560, Height: 720},
		},
		Client: ClientIdentity{PID: 99999999, UID: 1001, GID: 1002},
	})
	bridge.mu.Lock()
	tracked := bridge.surfaces["layer-stale"]
	tracked.UpdatedAt = time.Now().Add(-deadClientPruneAfter - time.Second)
	bridge.surfaces["layer-stale"] = tracked
	bridge.mu.Unlock()

	if got := bridge.ListSurfaces(); len(got) != 0 {
		t.Fatalf("surfaces = %+v, want stale layer-shell surface pruned", got)
	}
	if _, ok := bridge.stale["layer-stale"]; !ok {
		t.Fatalf("stale marker missing after prune")
	}
}

func TestGetLayoutPrunesOldXDGShellSurfaceWithDeadClient(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-stale",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Visible:     &visible,
			OutputID:    "HDMI-A-1",
			Geometry:    &SurfaceGeometry{Width: 1200, Height: 800},
		},
		Client: ClientIdentity{PID: 99999999, UID: 1001, GID: 1002},
	})
	bridge.mu.Lock()
	tracked := bridge.surfaces["view-stale"]
	tracked.UpdatedAt = time.Now().Add(-deadClientPruneAfter - time.Second)
	bridge.surfaces["view-stale"] = tracked
	bridge.mu.Unlock()

	layout := bridge.GetLayout().Layout
	if len(layout.Surfaces) != 0 {
		t.Fatalf("layout surfaces = %+v, want stale XDG surface pruned", layout.Surfaces)
	}
	if got := bridge.ListSurfaces(); len(got) != 0 {
		t.Fatalf("surfaces = %+v, want stale XDG surface pruned", got)
	}
	if _, ok := bridge.stale["view-stale"]; !ok {
		t.Fatalf("stale marker missing after XDG prune")
	}
}

func TestGetLayoutDerivesStableWorkspaceState(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-b",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Foot",
			Title:       "foot",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{X: 900, Y: 0, Width: 800, Height: 600},
			OutputID:    "HDMI-A-1",
			ZoneID:      "secondary",
			LayoutRole:  string(SurfaceLayoutRoleTiled),
		},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-a",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Title:       "Alacritty",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 800, Height: 600},
			OutputID:    "HDMI-A-1",
			ZoneID:      "primary",
			LayoutRole:  string(SurfaceLayoutRoleTiled),
		},
	})

	layout := bridge.GetLayout().Layout
	if layout.Mode != LayoutModeZones || layout.Revision == 0 {
		t.Fatalf("layout header = %+v", layout)
	}
	if len(layout.Surfaces) != 2 {
		t.Fatalf("surfaces = %+v", layout.Surfaces)
	}
	if layout.Surfaces[0].SurfaceID != "view-a" || layout.Surfaces[0].Label != "1" || layout.Surfaces[0].ZoneID != "primary" || layout.Surfaces[0].Participation != SurfaceLayoutRoleTiled {
		t.Fatalf("first surface = %+v", layout.Surfaces[0])
	}
	if len(layout.Workspaces) != 1 || layout.Workspaces[0].ID != "workspace-1" || !layout.Workspaces[0].Active {
		t.Fatalf("workspaces = %+v", layout.Workspaces)
	}
	if got := layout.Workspaces[0].SurfaceOrder; len(got) != 2 || got[0] != "view-a" || got[1] != "view-b" {
		t.Fatalf("surface order = %+v", got)
	}
}

func TestActivateWorkspaceCreatesIndependentWorkspaceState(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-a",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Title:       "Alacritty",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 800, Height: 600},
			OutputID:    "HDMI-A-1",
		},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventFocused,
		Surface: CompositorSurface{ID: "view-a", Visible: &visible},
	})

	response, err := bridge.ActivateWorkspace(WorkspaceActionRequest{WorkspaceID: "workspace-2"})
	if err != nil {
		t.Fatalf("activate workspace-2: %v", err)
	}
	if response.Decision != DecisionAccepted || response.WorkspaceID != "workspace-2" {
		t.Fatalf("activation response = %+v", response)
	}
	layout := bridge.GetLayout().Layout
	if len(layout.Workspaces) != 2 {
		t.Fatalf("workspaces after activation = %+v", layout.Workspaces)
	}
	if workspaceByID(layout, "workspace-1").Active || !workspaceByID(layout, "workspace-2").Active {
		t.Fatalf("active workspace state = %+v", layout.Workspaces)
	}
	if surfaceByID(layout, "view-a").Focused || surfaceByID(layout, "view-a").Visible {
		t.Fatalf("inactive workspace surface should not project focus or visibility: %+v", surfaceByID(layout, "view-a"))
	}

	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-b",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "foot",
			Title:       "foot",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 800, Height: 600},
			OutputID:    "HDMI-A-1",
		},
	})
	layout = bridge.GetLayout().Layout
	if got := workspaceByID(layout, "workspace-1").SurfaceOrder; len(got) != 1 || got[0] != "view-a" {
		t.Fatalf("workspace-1 order = %+v", got)
	}
	if got := workspaceByID(layout, "workspace-2").SurfaceOrder; len(got) != 1 || got[0] != "view-b" {
		t.Fatalf("workspace-2 order = %+v", got)
	}
	if surfaceByID(layout, "view-b").WorkspaceID != "workspace-2" || !surfaceByID(layout, "view-b").Visible {
		t.Fatalf("new active workspace surface = %+v", surfaceByID(layout, "view-b"))
	}

	if _, err := bridge.ActivateWorkspace(WorkspaceActionRequest{WorkspaceID: "workspace-1"}); err != nil {
		t.Fatalf("reactivate workspace-1: %v", err)
	}
	layout = bridge.GetLayout().Layout
	if !workspaceByID(layout, "workspace-1").Active || workspaceByID(layout, "workspace-2").Active {
		t.Fatalf("reactivated workspace state = %+v", layout.Workspaces)
	}
	if !surfaceByID(layout, "view-a").Visible || !surfaceByID(layout, "view-a").Focused {
		t.Fatalf("workspace-1 focus/visibility was not restored: %+v", surfaceByID(layout, "view-a"))
	}
	if surfaceByID(layout, "view-b").Visible {
		t.Fatalf("workspace-2 surface should be inactive after switching back: %+v", surfaceByID(layout, "view-b"))
	}
}

func TestActivateWorkspaceScopesVisibilityToOwningOutput(t *testing.T) {
	bridge := New(Config{})
	bridge.handlePluginEvent(pluginEvent{
		Type: PluginLayoutState,
		Layout: LayoutState{
			Mode:     LayoutModeZones,
			Revision: 11,
			Surfaces: []LayoutSurface{
				{
					SurfaceID:     "view-left",
					OutputID:      "HDMI-A-1",
					WorkspaceID:   "workspace-1",
					ZoneID:        zoneMaster,
					Mode:          LayoutModeZones,
					Participation: SurfaceLayoutRoleTiled,
					Visible:       true,
					Geometry:      &SurfaceGeometry{Width: 800, Height: 600},
				},
				{
					SurfaceID:     "view-right",
					OutputID:      "DP-1",
					WorkspaceID:   "workspace-2",
					ZoneID:        zoneMaster,
					Mode:          LayoutModeZones,
					Participation: SurfaceLayoutRoleTiled,
					Visible:       true,
					Geometry:      &SurfaceGeometry{X: 800, Width: 800, Height: 600},
				},
			},
			Workspaces: []LayoutWorkspace{
				{ID: "workspace-1", Name: "workspace 1", OutputID: "HDMI-A-1", Active: true, SurfaceOrder: []string{"view-left"}},
				{ID: "workspace-2", Name: "workspace 2", OutputID: "DP-1", Active: true, SurfaceOrder: []string{"view-right"}},
			},
		},
	})

	layout := bridge.GetLayout().Layout
	if !workspaceByID(layout, "workspace-1").Active || !workspaceByID(layout, "workspace-2").Active {
		t.Fatalf("initial active workspace state = %+v", layout.Workspaces)
	}
	if surfaceByID(layout, "view-left").OutputID != "HDMI-A-1" || surfaceByID(layout, "view-right").OutputID != "DP-1" {
		t.Fatalf("surface output identity missing: %+v", layout.Surfaces)
	}

	response, err := bridge.ActivateWorkspace(WorkspaceActionRequest{WorkspaceID: "workspace-3", OutputID: "DP-1"})
	if err != nil {
		t.Fatalf("activate workspace-3 on DP-1: %v", err)
	}
	if response.Decision != DecisionAccepted || response.WorkspaceID != "workspace-3" {
		t.Fatalf("activation response = %+v", response)
	}
	layout = bridge.GetLayout().Layout
	if !workspaceByID(layout, "workspace-1").Active || workspaceByID(layout, "workspace-2").Active || !workspaceByID(layout, "workspace-3").Active {
		t.Fatalf("output-scoped active workspace state = %+v", layout.Workspaces)
	}
	if workspaceByID(layout, "workspace-3").OutputID != "DP-1" {
		t.Fatalf("workspace-3 output identity = %+v", workspaceByID(layout, "workspace-3"))
	}
	if !surfaceByID(layout, "view-left").Visible || surfaceByID(layout, "view-right").Visible {
		t.Fatalf("activation should only hide surfaces on DP-1: %+v", layout.Surfaces)
	}
}

func TestGetLayoutUsesBackendLayoutStateWhenPluginProvidesIt(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-a",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Title:       "Alacritty",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{X: 96, Y: 66, Width: 804, Height: 634},
			OutputID:    "HDMI-A-1",
		},
	})
	bridge.handlePluginEvent(pluginEvent{
		Type: PluginLayoutState,
		Layout: LayoutState{
			Mode:     LayoutModeZones,
			Revision: 7,
			Surfaces: []LayoutSurface{
				{
					SurfaceID:     "view-a",
					Label:         "1",
					AppID:         "Alacritty",
					Title:         "Alacritty",
					OutputID:      "HDMI-A-1",
					WorkspaceID:   "workspace-1",
					ZoneID:        "primary",
					Mode:          LayoutModeZones,
					Participation: SurfaceLayoutRoleTiled,
					Focused:       true,
					Visible:       true,
					Geometry:      &SurfaceGeometry{X: 0, Y: 0, Width: 1280, Height: 1248},
				},
				{
					SurfaceID:     "view-b",
					Label:         "2",
					AppID:         "foot",
					Title:         "foot",
					OutputID:      "HDMI-A-1",
					WorkspaceID:   "workspace-1",
					ZoneID:        "secondary",
					Mode:          LayoutModeZones,
					Participation: SurfaceLayoutRoleTiled,
					Visible:       true,
					Geometry:      &SurfaceGeometry{X: 1280, Y: 0, Width: 1280, Height: 1248},
				},
			},
			Workspaces: []LayoutWorkspace{
				{
					ID:       "workspace-1",
					Name:     "workspace 1",
					OutputID: "HDMI-A-1",
					Active:   true,
					Zones: []LayoutZone{
						{ID: "primary", Name: "Primary", Kind: "work", SurfaceIDs: []string{"view-a"}},
						{ID: "secondary", Name: "Secondary", Kind: "work", SurfaceIDs: []string{"view-b"}},
					},
					SurfaceOrder: []string{"view-a", "view-b"},
				},
			},
		},
	})

	layout := bridge.GetLayout().Layout
	if layout.Mode != LayoutModeZones || layout.Revision != 7 {
		t.Fatalf("layout header = %+v", layout)
	}
	if len(layout.Surfaces) != 2 {
		t.Fatalf("surfaces = %+v", layout.Surfaces)
	}
	if layout.Surfaces[0].Geometry.X != 0 || layout.Surfaces[1].Geometry.X != 1280 {
		t.Fatalf("backend geometry was not preserved: %+v", layout.Surfaces)
	}
	if layout.Surfaces[0].Participation != SurfaceLayoutRoleTiled || layout.Surfaces[0].Floating {
		t.Fatalf("layout participation = %+v", layout.Surfaces[0])
	}
	surfaces := bridge.ListSurfaces()
	if len(surfaces) != 2 {
		t.Fatalf("tracked surfaces = %+v", surfaces)
	}
	if surfaces[0].LayoutRevision != 7 || surfaces[0].Geometry.X != 0 {
		t.Fatalf("tracked surface did not receive backend layout readback: %+v", surfaces[0])
	}
}

func TestBackendLayoutStatePreservesTrackedFocus(t *testing.T) {
	bridge := New(Config{})
	visible := true
	for _, id := range []string{"view-a", "view-b"} {
		bridge.handleSurfaceEvent(pluginEvent{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          id,
				SurfaceKind: SurfaceKindXDG,
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
				OutputID:    "HDMI-A-1",
			},
		})
	}
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventFocused,
		Surface: CompositorSurface{ID: "view-b", Visible: &visible},
	})

	bridge.handlePluginEvent(pluginEvent{
		Type: PluginLayoutState,
		Layout: LayoutState{
			Mode:     LayoutModeZones,
			Revision: 9,
			Surfaces: []LayoutSurface{
				{
					SurfaceID:     "view-a",
					WorkspaceID:   "workspace-1",
					ZoneID:        zoneMaster,
					Mode:          LayoutModeZones,
					Participation: SurfaceLayoutRoleTiled,
					Focused:       true,
					Visible:       true,
					Geometry:      &SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 500},
				},
				{
					SurfaceID:     "view-b",
					WorkspaceID:   "workspace-1",
					ZoneID:        zoneStack,
					Mode:          LayoutModeZones,
					Participation: SurfaceLayoutRoleTiled,
					Visible:       true,
					Geometry:      &SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 500},
				},
			},
		},
	})

	layout := bridge.GetLayout().Layout
	if len(layout.Surfaces) != 2 {
		t.Fatalf("layout surfaces = %+v", layout.Surfaces)
	}
	if layout.Surfaces[0].Focused || !layout.Surfaces[1].Focused {
		t.Fatalf("layout readback overwrote tracked focus: %+v", layout.Surfaces)
	}
	surfaces := bridge.ListSurfaces()
	if surfaces[0].Focused || !surfaces[1].Focused {
		t.Fatalf("tracked focus was not preserved: %+v", surfaces)
	}
}

func TestBackendLayoutStateDropsUnmappedSurface(t *testing.T) {
	bridge := New(Config{})
	bridge.handlePluginEvent(pluginEvent{
		Type: PluginLayoutState,
		Layout: LayoutState{
			Mode:     LayoutModeZones,
			Revision: 3,
			Surfaces: []LayoutSurface{
				{SurfaceID: "view-a", Visible: true, Geometry: &SurfaceGeometry{Width: 10, Height: 10}},
				{SurfaceID: "view-b", Visible: true, Geometry: &SurfaceGeometry{X: 10, Width: 10, Height: 10}},
			},
		},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventUnmapped,
		Surface: CompositorSurface{ID: "view-a"},
	})

	layout := bridge.GetLayout().Layout
	if len(layout.Surfaces) != 1 || layout.Surfaces[0].SurfaceID != "view-b" {
		t.Fatalf("layout surfaces after unmap = %+v", layout.Surfaces)
	}
	if layout.Revision <= 3 {
		t.Fatalf("layout revision did not advance after unmap: %+v", layout)
	}
	if got := layout.Workspaces[0].SurfaceOrder; len(got) != 1 || got[0] != "view-b" {
		t.Fatalf("surface order after unmap = %+v", got)
	}
}

func TestAssignSurfaceZonePlacesSurfaceThroughPlugin(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "layer-background",
			SurfaceKind: SurfaceKindLayer,
			AppID:       "io.agorade.ShellBackground",
			Role:        "background",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 2560, Height: 1344},
			OutputID:    "HDMI-A-1",
		},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "layer-panel",
			SurfaceKind: SurfaceKindLayer,
			AppID:       "io.agorade.ShellPanel",
			Role:        "panel",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 2560, Height: 96},
			OutputID:    "HDMI-A-1",
		},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-a",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Title:       "Alacritty",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{X: 96, Y: 66, Width: 804, Height: 634},
			OutputID:    "HDMI-A-1",
		},
	})

	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	go bridge.HandlePluginConn(pluginServer)

	decoder := json.NewDecoder(pluginClient)
	for range 2 {
		var initial map[string]any
		if err := decoder.Decode(&initial); err != nil {
			t.Fatalf("decode initial plugin message: %v", err)
		}
	}
	commandSeen := make(chan map[string]any, 1)
	go func() {
		var command map[string]any
		if err := decoder.Decode(&command); err == nil {
			commandSeen <- command
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		response, err := bridge.AssignSurfaceZone(SurfaceLayoutActionRequest{
			SurfaceID:     "view-a",
			ZoneID:        "secondary",
			WaitTimeoutMs: 1000,
		})
		if err != nil {
			t.Errorf("AssignSurfaceZone: %v", err)
			return
		}
		if response.Decision != DecisionAccepted || response.Layout == nil {
			t.Errorf("response = %+v", response)
		}
	}()

	var command map[string]any
	select {
	case command = <-commandSeen:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for place command")
	}
	if command["type"] != PluginPlaceSurface || command["surface_id"] != "view-a" {
		t.Fatalf("place command = %+v", command)
	}
	geometry, ok := command["geometry"].(map[string]any)
	if !ok || int(geometry["x"].(float64)) != 1280 || int(geometry["width"].(float64)) != 1280 || int(geometry["height"].(float64)) != 1344 {
		t.Fatalf("place geometry = %+v", command["geometry"])
	}
	if err := json.NewEncoder(pluginClient).Encode(map[string]any{
		"type":       PluginPlaceResponse,
		"request_id": command["request_id"],
		"surface_id": command["surface_id"],
		"ok":         true,
	}); err != nil {
		t.Fatalf("send place response: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for assign-zone response")
	}

	layout := bridge.GetLayout().Layout
	var placed LayoutSurface
	for _, surface := range layout.Surfaces {
		if surface.SurfaceID == "view-a" {
			placed = surface
			break
		}
	}
	if placed.ZoneID != "secondary" || placed.Geometry == nil || placed.Geometry.X != 1280 || placed.Participation != SurfaceLayoutRoleTiled {
		t.Fatalf("placed layout surface = %+v", placed)
	}
}

func TestAssignSurfaceZoneUsesPlannerGeometryAndBackendAck(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-a",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{X: 96, Y: 66, Width: 804, Height: 634},
			OutputID:    "HDMI-A-1",
		},
	})

	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	go bridge.HandlePluginConn(pluginServer)

	decoder := json.NewDecoder(pluginClient)
	for range 2 {
		var initial map[string]any
		if err := decoder.Decode(&initial); err != nil {
			t.Fatalf("decode initial plugin message: %v", err)
		}
	}
	commandSeen := make(chan map[string]any, 1)
	go func() {
		var command map[string]any
		if err := decoder.Decode(&command); err == nil {
			commandSeen <- command
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		response, err := bridge.AssignSurfaceZone(SurfaceLayoutActionRequest{
			SurfaceID:     "view-a",
			ZoneID:        "stack",
			Geometry:      &SurfaceGeometry{X: 601, Y: 20, Width: 389, Height: 378},
			WaitTimeoutMs: 1000,
		})
		if err != nil {
			t.Errorf("AssignSurfaceZone: %v", err)
			return
		}
		if response.Decision != DecisionAccepted || response.Layout == nil {
			t.Errorf("response = %+v", response)
		}
	}()

	var command map[string]any
	select {
	case command = <-commandSeen:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for place command")
	}
	geometry, ok := command["geometry"].(map[string]any)
	if !ok {
		t.Fatalf("place geometry = %+v", command["geometry"])
	}
	if int(geometry["x"].(float64)) != 601 || int(geometry["y"].(float64)) != 20 || int(geometry["width"].(float64)) != 389 || int(geometry["height"].(float64)) != 378 {
		t.Fatalf("place geometry = %+v", command["geometry"])
	}
	if err := json.NewEncoder(pluginClient).Encode(map[string]any{
		"type":       PluginPlaceResponse,
		"request_id": command["request_id"],
		"surface_id": command["surface_id"],
		"ok":         true,
		"geometry": map[string]any{
			"x":      610,
			"y":      30,
			"width":  380,
			"height": 360,
		},
	}); err != nil {
		t.Fatalf("send place response: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for assign-zone response")
	}

	layout := bridge.GetLayout().Layout
	var placed LayoutSurface
	for _, surface := range layout.Surfaces {
		if surface.SurfaceID == "view-a" {
			placed = surface
			break
		}
	}
	if placed.ZoneID != "stack" || placed.Geometry == nil || placed.Geometry.X != 610 || placed.Geometry.Width != 380 || placed.Participation != SurfaceLayoutRoleTiled {
		t.Fatalf("placed layout surface = %+v", placed)
	}
}

func TestAutoLayoutPlacesMappedSurfacesAndRelayoutsAfterUnmap(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	go bridge.HandlePluginConn(pluginServer)
	decoder := json.NewDecoder(pluginClient)
	encoder := json.NewEncoder(pluginClient)
	readInitialPluginMessages(t, decoder)

	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "layer-background",
			SurfaceKind: SurfaceKindLayer,
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 1200, Height: 800},
			OutputID:    "HDMI-A-1",
		},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "layer-panel",
			SurfaceKind: SurfaceKindLayer,
			Role:        "panel",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 1200, Height: 40},
			OutputID:    "HDMI-A-1",
		},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-a",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 800, Height: 600},
			OutputID:    "HDMI-A-1",
		},
	})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 1200, Height: 800}, SurfaceGeometry{X: 0, Y: 0, Width: 1200, Height: 800})

	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-b",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "foot",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 800, Height: 600},
			OutputID:    "HDMI-A-1",
		},
	})
	readPlaceAndAckNoWait(t, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 600, Height: 800}, SurfaceGeometry{X: 0, Y: 0, Width: 600, Height: 800})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 600, Y: 0, Width: 600, Height: 800}, SurfaceGeometry{X: 610, Y: 10, Width: 580, Height: 780})

	layout := bridge.GetLayout().Layout
	if layout.Mode != LayoutModeZones || len(layout.Surfaces) != 2 {
		t.Fatalf("layout after auto map = %+v", layout)
	}
	if layout.Settings.Rule != LayoutRuleMasterStack || layout.Settings.MasterCount != 1 || layout.Settings.MasterRatio != 0.5 {
		t.Fatalf("layout settings = %+v", layout.Settings)
	}
	if layout.Surfaces[0].SurfaceID != "view-a" || layout.Surfaces[0].ZoneID != "master" || layout.Surfaces[0].Geometry.Width != 600 {
		t.Fatalf("master surface = %+v", layout.Surfaces[0])
	}
	if layout.Surfaces[1].SurfaceID != "view-b" || layout.Surfaces[1].ZoneID != "stack" || layout.Surfaces[1].Geometry.X != 610 {
		t.Fatalf("stack surface should use backend ack geometry: %+v", layout.Surfaces[1])
	}

	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventUnmapped,
		Surface: CompositorSurface{ID: "view-b"},
	})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 1200, Height: 800}, SurfaceGeometry{X: 0, Y: 0, Width: 1200, Height: 800})

	layout = bridge.GetLayout().Layout
	if len(layout.Surfaces) != 1 || layout.Surfaces[0].SurfaceID != "view-a" || layout.Surfaces[0].Geometry.Width != 1200 {
		t.Fatalf("layout after auto unmap = %+v", layout)
	}
}

func TestReservedBottomUsesLayerShellWorkAreaReadback(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "layer-panel",
			SurfaceKind: SurfaceKindLayer,
			Role:        "panel",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 1200, Height: 40},
			OutputID:    "HDMI-A-1",
		},
	})

	bridge.mu.RLock()
	if got := bridge.reservedBottomHeightLocked("HDMI-A-1", 1200, 840); got != 40 {
		bridge.mu.RUnlock()
		t.Fatalf("panel-only reservation = %d, want 40", got)
	}
	bridge.mu.RUnlock()

	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "layer-background",
			SurfaceKind: SurfaceKindLayer,
			Role:        "background",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 1200, Height: 800},
			OutputID:    "HDMI-A-1",
		},
	})

	bridge.mu.RLock()
	if got := bridge.reservedBottomHeightLocked("HDMI-A-1", 1200, 800); got != 0 {
		bridge.mu.RUnlock()
		t.Fatalf("work-area readback reservation = %d, want 0", got)
	}
	bridge.mu.RUnlock()
}

func TestAutoLayoutUsesStableLayerShellOutputBounds(t *testing.T) {
	bridge := New(Config{})
	visible := true
	for _, event := range []pluginEvent{
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "layer-panel",
				SurfaceKind: SurfaceKindLayer,
				Role:        "panel",
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{Width: 2560, Height: 96},
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "layer-background",
				SurfaceKind: SurfaceKindLayer,
				Role:        "background",
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{Width: 2560, Height: 1344},
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "view-a",
				SurfaceKind: SurfaceKindXDG,
				Role:        "toplevel",
				Label:       "1",
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{X: 0, Y: 0, Width: 1318, Height: 1331},
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "view-b",
				SurfaceKind: SurfaceKindXDG,
				Role:        "toplevel",
				Label:       "2",
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{X: 1318, Y: 0, Width: 1318, Height: 665},
				OutputID:    "HDMI-A-1",
			},
		},
	} {
		bridge.handleSurfaceEvent(event)
	}
	bridge.mu.Lock()
	bridge.plugin = &pluginSession{}
	bridge.layoutMode = LayoutModeColumns
	bridge.layoutSettings = DefaultLayoutSettings()
	bridge.layoutSettings.Mode = LayoutModeColumns
	bridge.mu.Unlock()

	outputs := bridge.ListOutputs()
	if len(outputs) != 1 || outputs[0].PhysicalWidth != 2560 || outputs[0].PhysicalHeight != 1344 {
		t.Fatalf("outputs = %+v, want stable 2560x1344 layer-shell bounds", outputs)
	}

	placements := bridge.autoLayoutPlan()
	if len(placements) != 2 {
		t.Fatalf("placements = %+v, want two placements", placements)
	}
	if placements[0].Geometry != (SurfaceGeometry{X: 0, Y: 0, Width: 1280, Height: 1344}) {
		t.Fatalf("master placement = %+v, want stable 1280x1344 half of 2560", placements[0].Geometry)
	}
	if placements[1].Geometry != (SurfaceGeometry{X: 1280, Y: 0, Width: 1280, Height: 1344}) {
		t.Fatalf("stack placement = %+v, want stable 1280x1344 half of 2560", placements[1].Geometry)
	}
}

func TestUnmanagedXDGSurfaceIsTransientAndExcludedFromAutoLayout(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-unmanaged",
			SurfaceKind: SurfaceKindXDG,
			Role:        "unmanaged",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{X: 138, Y: 1394, Width: 70, Height: 33},
			OutputID:    "HDMI-A-1",
		},
	})

	surfaces := bridge.ListSurfaces()
	if len(surfaces) != 1 {
		t.Fatalf("surfaces = %+v, want unmanaged helper surface", surfaces)
	}
	if surfaces[0].LayoutRole != string(SurfaceLayoutRoleTransient) || surfaces[0].ZoneID != zoneTransient {
		t.Fatalf("unmanaged surface classification = role %q zone %q, want transient", surfaces[0].LayoutRole, surfaces[0].ZoneID)
	}
	if surfaces[0].PolicyClass != SurfacePolicyClassTransient || surfaces[0].PolicyReason == "" {
		t.Fatalf("unmanaged surface policy = class %q reason %q, want transient evidence", surfaces[0].PolicyClass, surfaces[0].PolicyReason)
	}
	if isAutoTileSurface(surfaces[0]) {
		t.Fatalf("unmanaged surface should not be auto-tile eligible: %+v", surfaces[0])
	}
}

func TestSurfacePolicyClassificationCoversWorkChromeAndDialogs(t *testing.T) {
	bridge := New(Config{})
	visible := true
	for _, event := range []pluginEvent{
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "layer-panel",
				SurfaceKind: SurfaceKindLayer,
				Role:        "panel",
				Visible:     &visible,
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "view-work",
				SurfaceKind: SurfaceKindXDG,
				Role:        "toplevel",
				Visible:     &visible,
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:              "view-dialog-parented",
				SurfaceKind:     SurfaceKindXDG,
				Role:            "modal-dialog",
				ParentSurfaceID: "view-work",
				Visible:         &visible,
				OutputID:        "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "view-file-chooser",
				SurfaceKind: SurfaceKindXDG,
				Role:        "file-chooser-dialog",
				Visible:     &visible,
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "view-menu",
				SurfaceKind: SurfaceKindXDG,
				Role:        "popup-menu",
				Visible:     &visible,
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "view-shell-status",
				SurfaceKind: SurfaceKindXDG,
				AppID:       "io.agorade.ShellStatus",
				Visible:     &visible,
				OutputID:    "HDMI-A-1",
			},
		},
	} {
		bridge.handleSurfaceEvent(event)
	}

	byID := map[string]TrackedSurface{}
	for _, surface := range bridge.ListSurfaces() {
		byID[surface.Surface.ID] = surface
	}

	assertPolicy := func(surfaceID string, wantRole SurfaceLayoutRole, wantClass SurfacePolicyClass) {
		t.Helper()
		surface, ok := byID[surfaceID]
		if !ok {
			t.Fatalf("surface %s not tracked; got %+v", surfaceID, byID)
		}
		if surface.LayoutRole != string(wantRole) || surface.PolicyClass != wantClass || surface.PolicyReason == "" {
			t.Fatalf("surface %s = role %q class %q reason %q, want role %q class %q", surfaceID, surface.LayoutRole, surface.PolicyClass, surface.PolicyReason, wantRole, wantClass)
		}
	}
	assertPolicy("layer-panel", SurfaceLayoutRoleTransient, SurfacePolicyClassShellChrome)
	assertPolicy("view-work", SurfaceLayoutRoleTiled, SurfacePolicyClassWork)
	assertPolicy("view-dialog-parented", SurfaceLayoutRoleTransient, SurfacePolicyClassTransient)
	assertPolicy("view-file-chooser", SurfaceLayoutRoleTransient, SurfacePolicyClassNoParent)
	assertPolicy("view-menu", SurfaceLayoutRoleTransient, SurfacePolicyClassNoParent)
	assertPolicy("view-shell-status", SurfaceLayoutRoleTransient, SurfacePolicyClassTransient)

	layout := bridge.GetLayout().Layout
	layoutByID := map[string]LayoutSurface{}
	for _, surface := range layout.Surfaces {
		layoutByID[surface.SurfaceID] = surface
	}
	if layoutByID["view-dialog-parented"].ParentSurfaceID != "view-work" || layoutByID["view-dialog-parented"].PolicyClass != SurfacePolicyClassTransient {
		t.Fatalf("parented dialog layout policy = %+v", layoutByID["view-dialog-parented"])
	}
	if layoutByID["view-file-chooser"].PolicyClass != SurfacePolicyClassNoParent || layoutByID["view-menu"].PolicyClass != SurfacePolicyClassNoParent {
		t.Fatalf("unparented transient layout policies = chooser %+v menu %+v", layoutByID["view-file-chooser"], layoutByID["view-menu"])
	}
}

func TestAutoLayoutKeepsStableOrderOnCompositorFocus(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	go bridge.HandlePluginConn(pluginServer)
	decoder := json.NewDecoder(pluginClient)
	encoder := json.NewEncoder(pluginClient)
	readInitialPluginMessages(t, decoder)

	visible := true
	for _, event := range []pluginEvent{
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "layer-background",
				SurfaceKind: SurfaceKindLayer,
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{Width: 1000, Height: 700},
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "view-a",
				SurfaceKind: SurfaceKindXDG,
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
				OutputID:    "HDMI-A-1",
			},
		},
	} {
		bridge.handleSurfaceEvent(event)
	}
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 700}, SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 700})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-b",
			SurfaceKind: SurfaceKindXDG,
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
			OutputID:    "HDMI-A-1",
		},
	})
	readPlaceAndAckNoWait(t, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 700}, SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 700})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 700}, SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 700})

	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventFocused,
		Surface: CompositorSurface{ID: "view-b", Visible: &visible},
	})
	assertNoPluginCommand(t, decoder)

	layout := bridge.GetLayout().Layout
	if layout.Surfaces[0].SurfaceID != "view-a" || layout.Surfaces[0].Focused || layout.Surfaces[0].ZoneID != "master" {
		t.Fatalf("master surface moved after compositor focus: %+v", layout.Surfaces)
	}
	if layout.Surfaces[1].SurfaceID != "view-b" || !layout.Surfaces[1].Focused || layout.Surfaces[1].ZoneID != "stack" {
		t.Fatalf("stack focus state was not updated in place: %+v", layout.Surfaces)
	}
}

func TestFocusSurfaceAckUpdatesFocusStateAndRequestsAutoLayout(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	go bridge.HandlePluginConn(pluginServer)
	decoder := json.NewDecoder(pluginClient)
	encoder := json.NewEncoder(pluginClient)
	readInitialPluginMessages(t, decoder)

	visible := true
	for _, id := range []string{"view-a", "view-b"} {
		bridge.surfaces[id] = TrackedSurface{
			Surface: CompositorSurface{
				ID:          id,
				SurfaceKind: SurfaceKindXDG,
				Visible:     &visible,
				OutputID:    "HDMI-A-1",
				WorkspaceID: "workspace-1",
				ZoneID:      zoneStack,
				LayoutMode:  string(LayoutModeZones),
				LayoutRole:  string(SurfaceLayoutRoleTiled),
			},
			Visible:     true,
			OutputID:    "HDMI-A-1",
			WorkspaceID: "workspace-1",
			ZoneID:      zoneStack,
			LayoutMode:  string(LayoutModeZones),
			LayoutRole:  string(SurfaceLayoutRoleTiled),
			Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
		}
	}
	bridge.surfaces["view-a"] = func(surface TrackedSurface) TrackedSurface {
		surface.Focused = true
		return surface
	}(bridge.surfaces["view-a"])

	done := make(chan SurfaceActionResponse, 1)
	errs := make(chan error, 1)
	go func() {
		response, err := bridge.FocusSurface(FocusSurfaceRequest{SurfaceID: "view-b", WaitTimeoutMs: 1000})
		if err != nil {
			errs <- err
			return
		}
		done <- response
	}()

	var command map[string]any
	if err := decoder.Decode(&command); err != nil {
		t.Fatalf("decode focus command: %v", err)
	}
	if command["type"] != PluginFocusSurface || command["surface_id"] != "view-b" {
		t.Fatalf("focus command = %+v", command)
	}
	if err := encoder.Encode(map[string]any{
		"type":       PluginFocusResponse,
		"request_id": command["request_id"],
		"surface_id": command["surface_id"],
		"ok":         true,
	}); err != nil {
		t.Fatalf("send focus response: %v", err)
	}

	select {
	case err := <-errs:
		t.Fatalf("FocusSurface: %v", err)
	case response := <-done:
		if response.Decision != DecisionAccepted || response.Surface == nil || !response.Surface.Focused {
			t.Fatalf("focus response = %+v", response)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for focus response")
	}
	surfaces := map[string]TrackedSurface{}
	for _, surface := range bridge.ListSurfaces() {
		surfaces[surface.Surface.ID] = surface
	}
	if surfaces["view-a"].Focused || !surfaces["view-b"].Focused {
		t.Fatalf("focus state = %+v", surfaces)
	}
}

func TestPromoteSurfaceSetsBridgeOwnedFocusCandidate(t *testing.T) {
	bridge := New(Config{})
	visible := true
	for _, id := range []string{"view-a", "view-b"} {
		bridge.surfaces[id] = TrackedSurface{
			Surface: CompositorSurface{
				ID:          id,
				SurfaceKind: SurfaceKindXDG,
				Visible:     &visible,
				OutputID:    "HDMI-A-1",
				WorkspaceID: "workspace-1",
				ZoneID:      zoneStack,
				LayoutMode:  string(LayoutModeZones),
				LayoutRole:  string(SurfaceLayoutRoleTiled),
			},
			Visible:     true,
			OutputID:    "HDMI-A-1",
			WorkspaceID: "workspace-1",
			ZoneID:      zoneStack,
			LayoutMode:  string(LayoutModeZones),
			LayoutRole:  string(SurfaceLayoutRoleTiled),
		}
	}
	bridge.surfaces["view-a"] = func(surface TrackedSurface) TrackedSurface {
		surface.Focused = true
		return surface
	}(bridge.surfaces["view-a"])

	response, err := bridge.PromoteSurface(SurfaceLayoutActionRequest{SurfaceID: "view-b"})
	if err != nil {
		t.Fatalf("PromoteSurface: %v", err)
	}
	if response.Decision != DecisionAccepted || response.Action != "surface.promote" {
		t.Fatalf("response = %+v", response)
	}
	surfaces := map[string]TrackedSurface{}
	for _, surface := range bridge.ListSurfaces() {
		surfaces[surface.Surface.ID] = surface
	}
	if surfaces["view-a"].Focused || !surfaces["view-b"].Focused {
		t.Fatalf("focus state = %+v", surfaces)
	}
	if surfaces["view-b"].ZoneID != zoneMaster || surfaces["view-b"].LayoutRole != string(SurfaceLayoutRoleTiled) {
		t.Fatalf("promoted layout state = %+v", surfaces["view-b"])
	}
}

func TestPromoteSurfaceSurvivesCompositorFocusReadback(t *testing.T) {
	bridge := New(Config{})
	visible := true
	for _, id := range []string{"view-a", "view-b"} {
		bridge.handleSurfaceEvent(pluginEvent{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          id,
				SurfaceKind: SurfaceKindXDG,
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
				OutputID:    "HDMI-A-1",
			},
		})
	}
	if _, err := bridge.PromoteSurface(SurfaceLayoutActionRequest{SurfaceID: "view-b"}); err != nil {
		t.Fatalf("PromoteSurface: %v", err)
	}
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventFocused,
		Surface: CompositorSurface{ID: "view-a", Visible: &visible},
	})

	layout := bridge.GetLayout().Layout
	byID := map[string]LayoutSurface{}
	for _, surface := range layout.Surfaces {
		byID[surface.SurfaceID] = surface
	}
	if byID["view-a"].Focused || !byID["view-b"].Focused {
		t.Fatalf("layout focus followed compositor readback instead of promote: %+v", layout.Surfaces)
	}
}

func TestAutoLayoutOrderUsesPlacementZonesOverStaleTrackedZones(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.surfaces["view-a"] = TrackedSurface{
		Surface: CompositorSurface{
			ID:          "view-a",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Visible:     &visible,
			WorkspaceID: "workspace-1",
			ZoneID:      zoneMaster,
			LayoutMode:  string(LayoutModeZones),
			LayoutRole:  string(SurfaceLayoutRoleTiled),
		},
		Visible:     true,
		WorkspaceID: "workspace-1",
		ZoneID:      zoneMaster,
		LayoutMode:  string(LayoutModeZones),
		LayoutRole:  string(SurfaceLayoutRoleTiled),
		Geometry:    &SurfaceGeometry{X: 0, Y: 0, Width: 600, Height: 700},
	}
	bridge.surfaces["view-b"] = TrackedSurface{
		Surface: CompositorSurface{
			ID:          "view-b",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "foot",
			Visible:     &visible,
			WorkspaceID: "workspace-1",
			ZoneID:      zoneMaster,
			LayoutMode:  string(LayoutModeZones),
			LayoutRole:  string(SurfaceLayoutRoleTiled),
		},
		Visible:     true,
		WorkspaceID: "workspace-1",
		ZoneID:      zoneMaster,
		LayoutMode:  string(LayoutModeZones),
		LayoutRole:  string(SurfaceLayoutRoleTiled),
		Geometry:    &SurfaceGeometry{X: 600, Y: 0, Width: 600, Height: 700},
	}

	bridge.applyAutoLayoutOrder([]autoLayoutPlacement{
		{SurfaceID: "view-a", WorkspaceID: "workspace-1", ZoneID: zoneStack, Geometry: SurfaceGeometry{X: 600, Y: 0, Width: 600, Height: 700}},
		{SurfaceID: "view-b", WorkspaceID: "workspace-1", ZoneID: zoneMaster, Geometry: SurfaceGeometry{X: 0, Y: 0, Width: 600, Height: 700}},
	})

	layout := bridge.GetLayout().Layout
	if len(layout.Surfaces) != 2 {
		t.Fatalf("layout surfaces = %+v", layout.Surfaces)
	}
	if layout.Surfaces[0].SurfaceID != "view-a" || layout.Surfaces[0].ZoneID != zoneStack {
		t.Fatalf("first surface should use placement stack zone despite stale tracked zone: %+v", layout.Surfaces[0])
	}
	if layout.Surfaces[1].SurfaceID != "view-b" || layout.Surfaces[1].ZoneID != zoneMaster {
		t.Fatalf("second surface should use placement master zone: %+v", layout.Surfaces[1])
	}
	zones := map[string][]string{}
	for _, zone := range layout.Workspaces[0].Zones {
		zones[zone.ID] = zone.SurfaceIDs
	}
	if got := zones[zoneMaster]; len(got) != 1 || got[0] != "view-b" {
		t.Fatalf("master zone membership = %+v, want only view-b", zones[zoneMaster])
	}
	if got := zones[zoneStack]; len(got) != 1 || got[0] != "view-a" {
		t.Fatalf("stack zone membership = %+v, want only view-a", zones[zoneStack])
	}
}

func TestNormalizeLayoutStateRemovesStaleZoneMembership(t *testing.T) {
	layout := LayoutState{
		Mode: LayoutModeZones,
		Surfaces: []LayoutSurface{
			{
				SurfaceID:     "view-a",
				WorkspaceID:   "workspace-1",
				ZoneID:        zoneMaster,
				Mode:          LayoutModeZones,
				Participation: SurfaceLayoutRoleTiled,
				Geometry:      &SurfaceGeometry{X: 0, Y: 0, Width: 600, Height: 700},
			},
			{
				SurfaceID:     "view-b",
				WorkspaceID:   "workspace-1",
				ZoneID:        zoneStack,
				Mode:          LayoutModeZones,
				Participation: SurfaceLayoutRoleTiled,
				Geometry:      &SurfaceGeometry{X: 600, Y: 0, Width: 600, Height: 700},
			},
		},
		Workspaces: []LayoutWorkspace{
			{
				ID:           "workspace-1",
				Name:         "workspace 1",
				Active:       true,
				SurfaceOrder: []string{"view-a", "view-a", "view-b"},
				Zones: []LayoutZone{
					{ID: zoneMaster, Name: zoneMaster, Kind: "work", SurfaceIDs: []string{"view-a", "view-b"}},
					{ID: zoneStack, Name: zoneStack, Kind: "work", SurfaceIDs: []string{"view-b", "view-b"}},
				},
			},
		},
	}

	normalizeLayoutState(&layout)

	if got := layout.Workspaces[0].SurfaceOrder; len(got) != 2 || got[0] != "view-a" || got[1] != "view-b" {
		t.Fatalf("surface order = %+v", got)
	}
	zones := map[string][]string{}
	for _, zone := range layout.Workspaces[0].Zones {
		zones[zone.ID] = zone.SurfaceIDs
	}
	if got := zones[zoneMaster]; len(got) != 1 || got[0] != "view-a" {
		t.Fatalf("master zone membership = %+v, want only view-a", zones[zoneMaster])
	}
	if got := zones[zoneStack]; len(got) != 1 || got[0] != "view-b" {
		t.Fatalf("stack zone membership = %+v, want only view-b", zones[zoneStack])
	}
}

func TestAutoLayoutPlacementDoesNotOverrideNewerFloatingDecision(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	go bridge.HandlePluginConn(pluginServer)
	decoder := json.NewDecoder(pluginClient)
	encoder := json.NewEncoder(pluginClient)
	readInitialPluginMessages(t, decoder)

	visible := true
	bridge.surfaces["view-a"] = TrackedSurface{
		Surface: CompositorSurface{
			ID:          "view-a",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "Alacritty",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
			OutputID:    "HDMI-A-1",
			WorkspaceID: "workspace-1",
			ZoneID:      zoneMaster,
			LayoutMode:  string(LayoutModeZones),
			LayoutRole:  string(SurfaceLayoutRoleTiled),
		},
		Visible:     true,
		OutputID:    "HDMI-A-1",
		WorkspaceID: "workspace-1",
		ZoneID:      zoneMaster,
		LayoutMode:  string(LayoutModeZones),
		LayoutRole:  string(SurfaceLayoutRoleTiled),
		Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
	}
	done := make(chan error, 1)
	go func() {
		request := SurfaceLayoutActionRequest{SurfaceID: "view-a", WorkspaceID: "workspace-1", ZoneID: zoneMaster, WaitTimeoutMs: 1000}
		_, err := bridge.placeSurfaceChecked(
			request,
			"layout.auto_tile",
			SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 600},
			zoneMaster,
			SurfaceLayoutRoleTiled,
			func(tracked TrackedSurface) bool {
				return isAutoTileSurface(tracked)
			},
			false,
		)
		done <- err
	}()

	var command map[string]any
	if err := decoder.Decode(&command); err != nil {
		t.Fatalf("decode place command: %v", err)
	}
	if command["type"] != PluginPlaceSurface || command["surface_id"] != "view-a" {
		t.Fatalf("place command = %+v", command)
	}
	bridge.mu.Lock()
	tracked := bridge.surfaces["view-a"]
	tracked.LayoutMode = string(LayoutModeFreeform)
	tracked.Surface.LayoutMode = tracked.LayoutMode
	tracked.LayoutRole = string(SurfaceLayoutRoleFloating)
	tracked.Surface.LayoutRole = tracked.LayoutRole
	tracked.ZoneID = zoneTransient
	tracked.Surface.ZoneID = tracked.ZoneID
	bridge.surfaces["view-a"] = tracked
	bridge.mu.Unlock()

	if err := encoder.Encode(map[string]any{
		"type":       PluginPlaceResponse,
		"request_id": command["request_id"],
		"surface_id": command["surface_id"],
		"ok":         true,
		"geometry":   SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 600},
	}); err != nil {
		t.Fatalf("send place response: %v", err)
	}
	select {
	case err := <-done:
		if class, _ := classifyError(err); class != ErrorSurfaceStale {
			t.Fatalf("placeSurfaceChecked err = %v, class = %s, want stale", err, class)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for placement result")
	}

	surfaces := bridge.ListSurfaces()
	if len(surfaces) != 1 || surfaces[0].LayoutRole != string(SurfaceLayoutRoleFloating) || surfaces[0].ZoneID != zoneTransient {
		t.Fatalf("surface state after stale placement = %+v", surfaces)
	}
}

func TestSetLayoutModeUpdatesStateAndFreeformDisablesAutoLayout(t *testing.T) {
	bridge := New(Config{})
	response, err := bridge.SetLayoutMode(SetLayoutModeRequest{Mode: LayoutModeFreeform})
	if err != nil {
		t.Fatalf("SetLayoutMode: %v", err)
	}
	if response.Decision != DecisionAccepted || response.Layout == nil || response.Layout.Mode != LayoutModeFreeform {
		t.Fatalf("response = %+v", response)
	}

	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	go bridge.HandlePluginConn(pluginServer)
	decoder := json.NewDecoder(pluginClient)
	readInitialPluginMessages(t, decoder)
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-freeform",
			SurfaceKind: SurfaceKindXDG,
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
			OutputID:    "HDMI-A-1",
		},
	})
	assertNoPluginCommand(t, decoder)
}

func TestLifecycleClassifiesEverydayAppsShellSurfacesAndDialogs(t *testing.T) {
	bridge := New(Config{})
	visible := true
	for _, surface := range []CompositorSurface{
		{
			ID:          "firefox",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "firefox",
			Role:        "toplevel",
			Visible:     &visible,
			OutputID:    "HDMI-A-1",
		},
		{
			ID:          "dolphin",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "org.kde.dolphin",
			Role:        "toplevel",
			Visible:     &visible,
			OutputID:    "HDMI-A-1",
		},
		{
			ID:          "launcher",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "io.agorade.ShellLauncher",
			Role:        "toplevel",
			Visible:     &visible,
			OutputID:    "HDMI-A-1",
		},
		{
			ID:          "dialog",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "firefox",
			Role:        "dialog",
			Visible:     &visible,
			OutputID:    "HDMI-A-1",
		},
	} {
		bridge.handleSurfaceEvent(pluginEvent{Type: PluginSurfaceEvent, Event: EventMapped, Surface: surface})
	}

	surfaces := map[string]TrackedSurface{}
	for _, surface := range bridge.ListSurfaces() {
		surfaces[surface.Surface.ID] = surface
	}
	for _, id := range []string{"firefox", "dolphin"} {
		surface := surfaces[id]
		if surface.LayoutRole != string(SurfaceLayoutRoleTiled) || surface.LayoutMode != string(LayoutModeZones) {
			t.Fatalf("%s classified as %+v, want tiled zones", id, surface)
		}
	}
	for _, id := range []string{"launcher", "dialog"} {
		surface := surfaces[id]
		if surface.LayoutRole != string(SurfaceLayoutRoleTransient) || surface.LayoutMode != string(LayoutModeFreeform) || surface.ZoneID != zoneTransient {
			t.Fatalf("%s classified as %+v, want transient freeform", id, surface)
		}
	}
}

func TestSetSurfaceFloatingEscapesAndReturnsToAutoLayout(t *testing.T) {
	bridge := New(Config{})
	pluginClient, pluginServer := net.Pipe()
	defer pluginClient.Close()
	go bridge.HandlePluginConn(pluginServer)
	decoder := json.NewDecoder(pluginClient)
	encoder := json.NewEncoder(pluginClient)
	readInitialPluginMessages(t, decoder)

	visible := true
	for _, event := range []pluginEvent{
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "layer-background",
				SurfaceKind: SurfaceKindLayer,
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{Width: 1000, Height: 600},
				OutputID:    "HDMI-A-1",
			},
		},
		{
			Type:  PluginSurfaceEvent,
			Event: EventMapped,
			Surface: CompositorSurface{
				ID:          "view-a",
				SurfaceKind: SurfaceKindXDG,
				AppID:       "Alacritty",
				Visible:     &visible,
				Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
				OutputID:    "HDMI-A-1",
			},
		},
	} {
		bridge.handleSurfaceEvent(event)
	}
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 600}, SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 600})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:  PluginSurfaceEvent,
		Event: EventMapped,
		Surface: CompositorSurface{
			ID:          "view-b",
			SurfaceKind: SurfaceKindXDG,
			AppID:       "firefox",
			Visible:     &visible,
			Geometry:    &SurfaceGeometry{Width: 500, Height: 500},
			OutputID:    "HDMI-A-1",
		},
	})
	readPlaceAndAckNoWait(t, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 600}, SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 600})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 600}, SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 600})

	enabled := true
	response, err := bridge.SetSurfaceFloating(SurfaceLayoutActionRequest{SurfaceID: "view-b", Floating: &enabled})
	if err != nil {
		t.Fatalf("SetSurfaceFloating(true): %v", err)
	}
	if response.Decision != DecisionAccepted || response.Surface == nil || response.Surface.LayoutRole != string(SurfaceLayoutRoleFloating) {
		t.Fatalf("floating response = %+v", response)
	}
	if response.Surface.PolicyClass != SurfacePolicyClassFloatingOverride || response.Surface.PolicyReason == "" {
		t.Fatalf("floating response policy = %+v, want explicit floating override", response.Surface)
	}
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 600}, SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 600})
	layout := bridge.GetLayout().Layout
	if len(layout.Surfaces) != 2 || layout.Surfaces[0].SurfaceID != "view-a" || layout.Surfaces[1].SurfaceID != "view-b" {
		t.Fatalf("floating surface should remain visible after tiled surfaces: %+v", layout.Surfaces)
	}
	if layout.Surfaces[1].Participation != SurfaceLayoutRoleFloating || layout.Surfaces[1].ZoneID != zoneTransient {
		t.Fatalf("floating layout surface = %+v", layout.Surfaces[1])
	}
	if layout.Surfaces[1].PolicyClass != SurfacePolicyClassFloatingOverride || layout.Surfaces[1].PolicyReason == "" {
		t.Fatalf("floating layout policy = %+v", layout.Surfaces[1])
	}
	surfaces := bridge.ListSurfaces()
	var floating TrackedSurface
	for _, surface := range surfaces {
		if surface.Surface.ID == "view-b" {
			floating = surface
			break
		}
	}
	if floating.LayoutRole != string(SurfaceLayoutRoleFloating) || floating.ZoneID != zoneTransient || floating.PolicyClass != SurfacePolicyClassFloatingOverride {
		t.Fatalf("floating surface state = %+v", floating)
	}

	enabled = false
	response, err = bridge.SetSurfaceFloating(SurfaceLayoutActionRequest{SurfaceID: "view-b", Floating: &enabled})
	if err != nil {
		t.Fatalf("SetSurfaceFloating(false): %v", err)
	}
	if response.Decision != DecisionAccepted {
		t.Fatalf("return-to-tiling response = %+v", response)
	}
	if response.Surface == nil || response.Surface.PolicyClass != SurfacePolicyClassWork {
		t.Fatalf("return-to-tiling policy = %+v, want work", response.Surface)
	}
	readPlaceAndAckNoWait(t, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 600}, SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 600})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 600}, SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 600})
	layout = bridge.GetLayout().Layout
	if len(layout.Surfaces) != 2 || layout.Surfaces[1].SurfaceID != "view-b" || layout.Surfaces[1].Participation != SurfaceLayoutRoleTiled {
		t.Fatalf("surface did not return to tiled layout: %+v", layout.Surfaces)
	}
	if layout.Surfaces[1].PolicyClass != SurfacePolicyClassWork {
		t.Fatalf("returned tiled layout policy = %+v", layout.Surfaces[1])
	}
}

func readInitialPluginMessages(t *testing.T, decoder *json.Decoder) {
	t.Helper()
	for range 2 {
		var initial map[string]any
		if err := decoder.Decode(&initial); err != nil {
			t.Fatalf("decode initial plugin message: %v", err)
		}
	}
}

func readPlaceAndAck(t *testing.T, bridge *Bridge, decoder *json.Decoder, encoder *json.Encoder, surfaceID string, want SurfaceGeometry, ack SurfaceGeometry) {
	t.Helper()
	readPlaceAndAckNoWait(t, decoder, encoder, surfaceID, want, ack)
	waitForLayoutGeometry(t, bridge, surfaceID, ack)
}

func readPlaceAndAckNoWait(t *testing.T, decoder *json.Decoder, encoder *json.Encoder, surfaceID string, want SurfaceGeometry, ack SurfaceGeometry) {
	t.Helper()
	commandCh := make(chan map[string]any, 1)
	go func() {
		var command map[string]any
		if err := decoder.Decode(&command); err == nil {
			commandCh <- command
		}
	}()
	var command map[string]any
	select {
	case command = <-commandCh:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for place command for %s", surfaceID)
	}
	if command["type"] != PluginPlaceSurface || command["surface_id"] != surfaceID {
		t.Fatalf("place command = %+v, want surface %s", command, surfaceID)
	}
	geometry, ok := command["geometry"].(map[string]any)
	if !ok {
		t.Fatalf("place geometry = %+v", command["geometry"])
	}
	got := SurfaceGeometry{
		X:      int(geometry["x"].(float64)),
		Y:      int(geometry["y"].(float64)),
		Width:  int(geometry["width"].(float64)),
		Height: int(geometry["height"].(float64)),
	}
	if got != want {
		t.Fatalf("place geometry = %+v, want %+v", got, want)
	}
	if err := encoder.Encode(map[string]any{
		"type":       PluginPlaceResponse,
		"request_id": command["request_id"],
		"surface_id": command["surface_id"],
		"ok":         true,
		"geometry": map[string]any{
			"x":      ack.X,
			"y":      ack.Y,
			"width":  ack.Width,
			"height": ack.Height,
		},
	}); err != nil {
		t.Fatalf("send place response: %v", err)
	}
}

func waitForLayoutGeometry(t *testing.T, bridge *Bridge, surfaceID string, want SurfaceGeometry) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		layout := bridge.GetLayout().Layout
		for _, surface := range layout.Surfaces {
			if surface.SurfaceID != surfaceID || surface.Geometry == nil {
				continue
			}
			got := *surface.Geometry
			if got == want {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for layout geometry %s = %+v", surfaceID, want)
}

func assertNoPluginCommand(t *testing.T, decoder *json.Decoder) {
	t.Helper()
	commandCh := make(chan map[string]any, 1)
	go func() {
		var command map[string]any
		if err := decoder.Decode(&command); err == nil {
			commandCh <- command
		}
	}()
	select {
	case command := <-commandCh:
		t.Fatalf("unexpected plugin command: %+v", command)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestLayoutSurfaceActionsValidateStaleBeforeUnsupportedBackend(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMapped,
		Surface: CompositorSurface{ID: "view-stale", SurfaceKind: SurfaceKindXDG, Visible: &visible},
	})
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventUnmapped,
		Surface: CompositorSurface{ID: "view-stale"},
	})

	_, err := bridge.MoveResizeSurface(SurfaceLayoutActionRequest{
		SurfaceID: "view-stale",
		Geometry:  &SurfaceGeometry{Width: 800, Height: 600},
	})
	class, _ := classifyError(err)
	if class != ErrorSurfaceStale {
		t.Fatalf("class = %q, want %q; err=%v", class, ErrorSurfaceStale, err)
	}
}

func TestLayoutSurfaceActionsReturnUnsupportedForValidSurface(t *testing.T) {
	bridge := New(Config{})
	visible := true
	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventMapped,
		Surface: CompositorSurface{ID: "view-live", SurfaceKind: SurfaceKindXDG, Visible: &visible},
	})

	response, err := bridge.TileSurface(SurfaceLayoutActionRequest{SurfaceID: "view-live", ZoneID: "primary"})
	class, _ := classifyError(err)
	if class != ErrorBackendUnsupported {
		t.Fatalf("class = %q, want %q; err=%v", class, ErrorBackendUnsupported, err)
	}
	if response.Action != "surface.tile" || response.SurfaceID != "view-live" || response.Decision != "unsupported" || response.Surface == nil {
		t.Fatalf("response = %+v", response)
	}
	if response.Surface.PolicyClass != SurfacePolicyClassBackendLimited || response.Surface.PolicyReason == "" {
		t.Fatalf("unsupported response policy = %+v, want backend-limited evidence", response.Surface)
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

func workspaceByID(layout LayoutState, workspaceID string) LayoutWorkspace {
	for _, workspace := range layout.Workspaces {
		if workspace.ID == workspaceID {
			return workspace
		}
	}
	return LayoutWorkspace{}
}

func surfaceByID(layout LayoutState, surfaceID string) LayoutSurface {
	for _, surface := range layout.Surfaces {
		if surface.SurfaceID == surfaceID {
			return surface
		}
	}
	return LayoutSurface{}
}

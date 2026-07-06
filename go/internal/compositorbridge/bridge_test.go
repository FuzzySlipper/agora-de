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
	if !ok || int(geometry["x"].(float64)) != 1280 || int(geometry["width"].(float64)) != 1280 || int(geometry["height"].(float64)) != 1248 {
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
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 1200, Height: 760}, SurfaceGeometry{X: 0, Y: 0, Width: 1200, Height: 760})

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
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 600, Height: 760}, SurfaceGeometry{X: 0, Y: 0, Width: 600, Height: 760})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 600, Y: 0, Width: 600, Height: 760}, SurfaceGeometry{X: 610, Y: 10, Width: 580, Height: 740})

	layout := bridge.GetLayout().Layout
	if layout.Mode != LayoutModeZones || len(layout.Surfaces) != 2 {
		t.Fatalf("layout after auto map = %+v", layout)
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
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 1200, Height: 760}, SurfaceGeometry{X: 0, Y: 0, Width: 1200, Height: 760})

	layout = bridge.GetLayout().Layout
	if len(layout.Surfaces) != 1 || layout.Surfaces[0].SurfaceID != "view-a" || layout.Surfaces[0].Geometry.Width != 1200 {
		t.Fatalf("layout after auto unmap = %+v", layout)
	}
}

func TestAutoLayoutPromotesFocusedSurfaceToMaster(t *testing.T) {
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
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 700}, SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 700})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 700}, SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 700})

	bridge.handleSurfaceEvent(pluginEvent{
		Type:    PluginSurfaceEvent,
		Event:   EventFocused,
		Surface: CompositorSurface{ID: "view-b", Visible: &visible},
	})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 700}, SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 700})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 700}, SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 700})

	layout := bridge.GetLayout().Layout
	if layout.Surfaces[0].SurfaceID != "view-b" || !layout.Surfaces[0].Focused || layout.Surfaces[0].ZoneID != "master" {
		t.Fatalf("focused surface was not promoted to master: %+v", layout.Surfaces)
	}
	if layout.Surfaces[1].SurfaceID != "view-a" || layout.Surfaces[1].ZoneID != "stack" {
		t.Fatalf("previous master was not moved to stack: %+v", layout.Surfaces)
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
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 600}, SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 600})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 600}, SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 600})

	enabled := true
	response, err := bridge.SetSurfaceFloating(SurfaceLayoutActionRequest{SurfaceID: "view-b", Floating: &enabled})
	if err != nil {
		t.Fatalf("SetSurfaceFloating(true): %v", err)
	}
	if response.Decision != DecisionAccepted || response.Surface == nil || response.Surface.LayoutRole != string(SurfaceLayoutRoleFloating) {
		t.Fatalf("floating response = %+v", response)
	}
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 600}, SurfaceGeometry{X: 0, Y: 0, Width: 1000, Height: 600})
	layout := bridge.GetLayout().Layout
	if len(layout.Surfaces) != 2 || layout.Surfaces[0].SurfaceID != "view-a" || layout.Surfaces[1].SurfaceID != "view-b" {
		t.Fatalf("floating surface should remain visible after tiled surfaces: %+v", layout.Surfaces)
	}
	if layout.Surfaces[1].Participation != SurfaceLayoutRoleFloating || layout.Surfaces[1].ZoneID != zoneTransient {
		t.Fatalf("floating layout surface = %+v", layout.Surfaces[1])
	}
	surfaces := bridge.ListSurfaces()
	var floating TrackedSurface
	for _, surface := range surfaces {
		if surface.Surface.ID == "view-b" {
			floating = surface
			break
		}
	}
	if floating.LayoutRole != string(SurfaceLayoutRoleFloating) || floating.ZoneID != zoneTransient {
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
	readPlaceAndAck(t, bridge, decoder, encoder, "view-a", SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 600}, SurfaceGeometry{X: 0, Y: 0, Width: 500, Height: 600})
	readPlaceAndAck(t, bridge, decoder, encoder, "view-b", SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 600}, SurfaceGeometry{X: 500, Y: 0, Width: 500, Height: 600})
	layout = bridge.GetLayout().Layout
	if len(layout.Surfaces) != 2 || layout.Surfaces[1].SurfaceID != "view-b" || layout.Surfaces[1].Participation != SurfaceLayoutRoleTiled {
		t.Fatalf("surface did not return to tiled layout: %+v", layout.Surfaces)
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
	waitForLayoutGeometry(t, bridge, surfaceID, ack)
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

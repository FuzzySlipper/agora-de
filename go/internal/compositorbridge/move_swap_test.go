package compositorbridge

import (
	"strings"
	"testing"
)

func seedMoveSwapBridge() *Bridge {
	bridge := New(Config{})
	bridge.layoutMode = LayoutModeZones
	bridge.layoutSettings.MasterCount = 1
	visible := true
	for _, id := range []string{"view-a", "view-b", "view-c", "view-d"} {
		bridge.surfaces[id] = TrackedSurface{
			Surface: CompositorSurface{
				ID:          id,
				SurfaceKind: SurfaceKindXDG,
				Visible:     &visible,
				OutputID:    "HDMI-A-1",
				WorkspaceID: "workspace-1",
				LayoutMode:  string(LayoutModeZones),
				LayoutRole:  string(SurfaceLayoutRoleTiled),
			},
			Visible:     true,
			OutputID:    "HDMI-A-1",
			WorkspaceID: "workspace-1",
			LayoutMode:  string(LayoutModeZones),
			LayoutRole:  string(SurfaceLayoutRoleTiled),
		}
	}
	bridge.surfaceOrder = []string{"view-a", "view-b", "view-c", "view-d"}
	return bridge
}

func TestOrderedActiveTiledSurfaceIDsDefaultsToSurfaceOrder(t *testing.T) {
	bridge := seedMoveSwapBridge()
	bridge.mu.Lock()
	ids := bridge.orderedActiveTiledSurfaceIDsLocked()
	bridge.mu.Unlock()
	if got, want := strings.Join(ids, ","), "view-a,view-b,view-c,view-d"; got != want {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestReorderMoveLeftPromotesStackWindowToMaster(t *testing.T) {
	bridge := seedMoveSwapBridge()
	bridge.mu.Lock()
	bridge.reorderSurfaceLocked("view-c", "left")
	ids := append([]string(nil), bridge.surfaceOrder...)
	bridge.mu.Unlock()
	if got, want := strings.Join(ids, ","), "view-c,view-a,view-b,view-d"; got != want {
		t.Fatalf("after move left = %v, want %v", got, want)
	}
}

func TestReorderMoveRightDemotesMasterToStack(t *testing.T) {
	bridge := seedMoveSwapBridge()
	bridge.mu.Lock()
	bridge.reorderSurfaceLocked("view-a", "right")
	ids := append([]string(nil), bridge.surfaceOrder...)
	bridge.mu.Unlock()
	if got, want := strings.Join(ids, ","), "view-b,view-a,view-c,view-d"; got != want {
		t.Fatalf("after move right = %v, want %v", got, want)
	}
}

func TestReorderMoveUpDownSwapsNeighbours(t *testing.T) {
	bridge := seedMoveSwapBridge()
	bridge.mu.Lock()
	bridge.reorderSurfaceLocked("view-c", "down")
	ids := append([]string(nil), bridge.surfaceOrder...)
	bridge.mu.Unlock()
	if got, want := strings.Join(ids, ","), "view-a,view-b,view-d,view-c"; got != want {
		t.Fatalf("after move down = %v, want %v", got, want)
	}

	bridge.mu.Lock()
	bridge.reorderSurfaceLocked("view-c", "up")
	ids = append([]string(nil), bridge.surfaceOrder...)
	bridge.mu.Unlock()
	if got, want := strings.Join(ids, ","), "view-a,view-b,view-c,view-d"; got != want {
		t.Fatalf("after move up = %v, want %v", got, want)
	}
}

func TestSwapWithMasterExchangesPositions(t *testing.T) {
	bridge := seedMoveSwapBridge()
	bridge.mu.Lock()
	bridge.swapWithMasterLocked("view-c")
	ids := append([]string(nil), bridge.surfaceOrder...)
	bridge.mu.Unlock()
	if got, want := strings.Join(ids, ","), "view-c,view-b,view-a,view-d"; got != want {
		t.Fatalf("after swap master = %v, want %v", got, want)
	}
}

func TestMoveSurfaceRejectsInvalidDirection(t *testing.T) {
	bridge := seedMoveSwapBridge()
	_, err := bridge.MoveSurface(MoveSurfaceRequest{SurfaceID: "view-b", Direction: "sideways"})
	if err == nil {
		t.Fatal("expected error for invalid direction")
	}
	class, _ := classifyError(err)
	if class != ErrorInvalidRequest {
		t.Fatalf("error class = %v, want %v", class, ErrorInvalidRequest)
	}
}

func TestMoveSurfaceAcceptsAndReorders(t *testing.T) {
	bridge := seedMoveSwapBridge()
	response, err := bridge.MoveSurface(MoveSurfaceRequest{SurfaceID: "view-c", Direction: "left"})
	if err != nil {
		t.Fatalf("MoveSurface: %v", err)
	}
	if response.Decision != DecisionAccepted || response.Action != "surface.move" {
		t.Fatalf("response = %+v", response)
	}
	bridge.mu.RLock()
	got := strings.Join(bridge.surfaceOrder, ",")
	bridge.mu.RUnlock()
	if want := "view-c,view-a,view-b,view-d"; got != want {
		t.Fatalf("surfaceOrder = %v, want %v", got, want)
	}
}

func TestSwapMasterSurfaceAcceptsAndReorders(t *testing.T) {
	bridge := seedMoveSwapBridge()
	response, err := bridge.SwapMasterSurface(SurfaceLayoutActionRequest{SurfaceID: "view-d"})
	if err != nil {
		t.Fatalf("SwapMasterSurface: %v", err)
	}
	if response.Decision != DecisionAccepted || response.Action != "surface.swap_master" {
		t.Fatalf("response = %+v", response)
	}
	bridge.mu.RLock()
	got := strings.Join(bridge.surfaceOrder, ",")
	bridge.mu.RUnlock()
	if want := "view-d,view-b,view-c,view-a"; got != want {
		t.Fatalf("surfaceOrder = %v, want %v", got, want)
	}
}

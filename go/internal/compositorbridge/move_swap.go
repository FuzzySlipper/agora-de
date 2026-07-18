package compositorbridge

import (
	"fmt"
	"strings"
	"time"
)

// This file implements the move/swap layout primitives (#5724): a mutable
// surface order that the auto-layout planner honors, plus the MoveSurface
// (directional) and SwapMasterSurface handlers. The contract types live in the
// Rust protocol crate (LayoutActionKind::MoveDirection / SwapMaster); the Go
// bridge is the runtime planner (auto_layout.go) that turns an order into real
// geometry pushed to the compositor.

// effectiveMasterCountLocked mirrors autoLayoutPlan's nmaster clamp.
func (bridge *Bridge) effectiveMasterCountLocked(total int) int {
	n := bridge.layoutSettings.MasterCount
	if n <= 0 {
		n = 1
	}
	if n > total {
		n = total
	}
	return n
}

// activeAutoTileSurfacesLocked returns the auto-tile surfaces on the active
// workspace (same filter autoLayoutPlan applies), unordered.
func (bridge *Bridge) activeAutoTileSurfacesLocked() []TrackedSurface {
	active := bridge.activeWorkspaceIDLocked()
	surfaces := make([]TrackedSurface, 0, len(bridge.surfaces))
	for _, surface := range bridge.surfaces {
		if !isAutoTileSurface(surface) {
			continue
		}
		if firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, active) != active {
			continue
		}
		surfaces = append(surfaces, surface)
	}
	return surfaces
}

// orderedActiveTiledSurfaceIDsLocked returns the active-workspace auto-tile
// surface IDs in the order the planner renders them: surfaces listed in
// bridge.surfaceOrder first (in listed order), then any remaining surfaces by
// the legacy promoted-first + label/id fallback.
func (bridge *Bridge) orderedActiveTiledSurfaceIDsLocked() []string {
	surfaces := bridge.activeAutoTileSurfacesLocked()
	if len(surfaces) == 0 {
		return nil
	}
	rank := make(map[string]int, len(bridge.surfaceOrder))
	for index, id := range bridge.surfaceOrder {
		if _, ok := rank[id]; !ok {
			rank[id] = index
		}
	}
	promoted := bridge.promotedSurfaceID
	ordered := make([]TrackedSurface, len(surfaces))
	copy(ordered, surfaces)
	sortStableByOrder(ordered, rank, promoted)
	ids := make([]string, len(ordered))
	for index, surface := range ordered {
		ids[index] = surface.Surface.ID
	}
	return ids
}

// sortStableByOrder orders surfaces by surfaceOrder rank (absent => after),
// falling back to promoted-first then label/id for surfaces with no rank.
func sortStableByOrder(surfaces []TrackedSurface, rank map[string]int, promoted string) {
	insertionSortStable(surfaces, func(i, j int) bool {
		ri, oki := rank[surfaces[i].Surface.ID]
		rj, okj := rank[surfaces[j].Surface.ID]
		if oki && okj {
			return ri < rj
		}
		if oki != okj {
			return oki // ordered surfaces precede unordered ones
		}
		if promoted != "" && (surfaces[i].Surface.ID == promoted || surfaces[j].Surface.ID == promoted) {
			return surfaces[i].Surface.ID == promoted
		}
		left := firstNonEmpty(surfaces[i].Surface.Label, surfaces[i].Surface.ID)
		right := firstNonEmpty(surfaces[j].Surface.Label, surfaces[j].Surface.ID)
		if left == right {
			return surfaces[i].Surface.ID < surfaces[j].Surface.ID
		}
		return left < right
	})
}

// insertionSortStable is a stable in-place sort by a less function. The surface
// counts here are tiny (a workspace's windows), so insertion sort is fine and
// avoids importing sort into this hot helper.
func insertionSortStable(s []TrackedSurface, less func(i, j int) bool) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// commitSurfaceOrder rewrites bridge.surfaceOrder from an explicit id list,
// pruned to currently-active surfaces, and clears the legacy promoted hint.
func (bridge *Bridge) commitSurfaceOrder(ids []string) {
	present := make(map[string]bool, len(bridge.surfaces))
	for _, surface := range bridge.activeAutoTileSurfacesLocked() {
		present[surface.Surface.ID] = true
	}
	pruned := make([]string, 0, len(ids))
	for _, id := range ids {
		if present[id] {
			pruned = append(pruned, id)
		}
	}
	// append any active surfaces missing from ids (stable label/id order)
	seen := make(map[string]bool, len(pruned))
	for _, id := range pruned {
		seen[id] = true
	}
	for _, surface := range bridge.activeAutoTileSurfacesLocked() {
		if !seen[surface.Surface.ID] {
			pruned = append(pruned, surface.Surface.ID)
		}
	}
	bridge.surfaceOrder = pruned
	bridge.promotedSurfaceID = ""
}

// reorderSurfaceLocked moves surfaceID one step in direction within the active
// layout order and commits the new order. left/right cross the master/stack
// column boundary (using the live master count); up/down swap with the neighbor.
func (bridge *Bridge) reorderSurfaceLocked(surfaceID, direction string) {
	ids := bridge.orderedActiveTiledSurfaceIDsLocked()
	if len(ids) <= 1 {
		bridge.commitSurfaceOrder(ids)
		return
	}
	current := indexOfString(ids, surfaceID)
	if current < 0 {
		bridge.commitSurfaceOrder(ids)
		return
	}
	nmaster := bridge.effectiveMasterCountLocked(len(ids))
	target := current
	switch direction {
	case "up":
		if current > 0 {
			target = current - 1
		}
	case "down":
		if current < len(ids)-1 {
			target = current + 1
		}
	case "left":
		if current >= nmaster {
			target = nmaster - 1
			if target < 0 {
				target = 0
			}
		}
	case "right":
		if current < nmaster && nmaster < len(ids) {
			target = nmaster
		}
	}
	if target != current {
		moved := ids[current]
		ids = append(ids[:current], ids[current+1:]...)
		if target > len(ids) {
			target = len(ids)
		}
		ids = append(ids[:target], append([]string{moved}, ids[target:]...)...)
	}
	bridge.commitSurfaceOrder(ids)
}

// swapWithMasterLocked exchanges surfaceID with the master (index 0).
func (bridge *Bridge) swapWithMasterLocked(surfaceID string) {
	ids := bridge.orderedActiveTiledSurfaceIDsLocked()
	if current := indexOfString(ids, surfaceID); current > 0 {
		ids[0], ids[current] = ids[current], ids[0]
	}
	bridge.commitSurfaceOrder(ids)
}

func indexOfString(values []string, want string) int {
	for index, value := range values {
		if value == want {
			return index
		}
	}
	return -1
}

// MoveSurface moves a surface one step in the given direction within the active
// layout order, then re-plans. Direction is a MoveDirection wire name
// (left|right|up|down).
func (bridge *Bridge) MoveSurface(request MoveSurfaceRequest) (LayoutActionResponse, error) {
	surface, err := bridge.requireWorkSurface(request.SurfaceID, "surface.move")
	if err != nil {
		return LayoutActionResponse{}, err
	}
	direction := strings.TrimSpace(strings.ToLower(request.Direction))
	switch direction {
	case "left", "right", "up", "down":
	default:
		return LayoutActionResponse{}, classifiedError{
			class:   ErrorInvalidRequest,
			message: fmt.Sprintf("invalid direction %q (left|right|up|down)", request.Direction),
		}
	}
	bridge.mu.Lock()
	bridge.reorderSurfaceLocked(request.SurfaceID, direction)
	bridge.focusActiveWorkspaceLocked(request.SurfaceID, surface)
	bridge.layoutSeq++
	bridge.updateBackendLayoutFocusLocked(request.SurfaceID)
	layout := bridge.layoutLocked()
	bridge.mu.Unlock()
	bridge.requestAutoLayout("surface_move")
	return LayoutActionResponse{
		Action:    "surface.move",
		SurfaceID: request.SurfaceID,
		Decision:  DecisionAccepted,
		Reason:    "surface moved by layout authority",
		Layout:    &layout,
	}, nil
}

// SwapMasterSurface exchanges the focused/command surface with the master
// (index 0), then re-plans.
func (bridge *Bridge) SwapMasterSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	surface, err := bridge.requireWorkSurface(request.SurfaceID, "surface.swap_master")
	if err != nil {
		return LayoutActionResponse{}, err
	}
	bridge.mu.Lock()
	bridge.swapWithMasterLocked(request.SurfaceID)
	bridge.focusActiveWorkspaceLocked(request.SurfaceID, surface)
	bridge.layoutSeq++
	bridge.updateBackendLayoutFocusLocked(request.SurfaceID)
	layout := bridge.layoutLocked()
	bridge.mu.Unlock()
	bridge.requestAutoLayout("surface_swap_master")
	return LayoutActionResponse{
		Action:    "surface.swap_master",
		SurfaceID: request.SurfaceID,
		Decision:  DecisionAccepted,
		Reason:    "surface swapped with master by layout authority",
		Layout:    &layout,
	}, nil
}

// SetSurfaceOrder sets the active workspace's planning order to the given ids
// (best-effort: pruned to active tiled surfaces, missing ones appended). Used by
// layout restore to reproduce a saved arrangement.
func (bridge *Bridge) SetSurfaceOrder(request SetSurfaceOrderRequest) (LayoutActionResponse, error) {
	bridge.mu.Lock()
	bridge.commitSurfaceOrder(request.SurfaceIDs)
	bridge.layoutSeq++
	layout := bridge.layoutLocked()
	bridge.mu.Unlock()
	bridge.requestAutoLayout("set_surface_order")
	return LayoutActionResponse{
		Action:   "set_surface_order",
		Decision: DecisionAccepted,
		Reason:   "surface order set by layout authority",
		Layout:   &layout,
	}, nil
}

// focusActiveWorkspaceLocked marks surfaceID focused among the active
// workspace's surfaces. Shared by MoveSurface/SwapMasterSurface.
func (bridge *Bridge) focusActiveWorkspaceLocked(surfaceID string, surface TrackedSurface) {
	targetWorkspaceID := firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, bridge.activeWorkspaceIDLocked())
	for id, tracked := range bridge.surfaces {
		if tracked.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		if firstNonEmpty(tracked.WorkspaceID, tracked.Surface.WorkspaceID, bridge.activeWorkspaceIDLocked()) != targetWorkspaceID {
			continue
		}
		tracked.Focused = id == surfaceID
		if id == surfaceID {
			tracked.LayoutMode = string(bridge.tiledLayoutModeLocked())
			tracked.Surface.LayoutMode = tracked.LayoutMode
			tracked.LayoutRole = string(SurfaceLayoutRoleTiled)
			tracked.Surface.LayoutRole = tracked.LayoutRole
			tracked.LayoutRevision = bridge.layoutSeq + 1
			tracked.UpdatedAt = time.Now()
		}
		bridge.surfaces[id] = tracked
	}
}

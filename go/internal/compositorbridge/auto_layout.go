package compositorbridge

import (
	"fmt"
	"log"
	"sort"
)

type autoLayoutPlacement struct {
	SurfaceID   string
	WorkspaceID string
	ZoneID      string
	Geometry    SurfaceGeometry
}

func (bridge *Bridge) requestAutoLayout(reason string) {
	bridge.mu.Lock()
	if bridge.plugin == nil || bridge.layoutMode == LayoutModeFreeform {
		bridge.mu.Unlock()
		return
	}
	bridge.autoLayoutSeq++
	if bridge.autoLayoutRunning {
		bridge.mu.Unlock()
		return
	}
	bridge.autoLayoutRunning = true
	bridge.mu.Unlock()
	go bridge.autoLayoutWorker(reason)
}

func (bridge *Bridge) autoLayoutWorker(reason string) {
	for {
		bridge.mu.RLock()
		targetSeq := bridge.autoLayoutSeq
		bridge.mu.RUnlock()
		if err := bridge.applyAutoLayoutOnce(reason, targetSeq); err != nil {
			log.Printf("auto layout: %v", err)
		}
		bridge.mu.Lock()
		if bridge.autoLayoutSeq == targetSeq {
			bridge.autoLayoutRunning = false
			bridge.mu.Unlock()
			return
		}
		bridge.mu.Unlock()
	}
}

func (bridge *Bridge) applyAutoLayoutOnce(reason string, targetSeq uint64) error {
	placements := bridge.autoLayoutPlan()
	applied := make([]autoLayoutPlacement, 0, len(placements))
	for _, placement := range placements {
		if !bridge.shouldApplyAutoLayoutPlacement(placement.SurfaceID) {
			continue
		}
		request := SurfaceLayoutActionRequest{
			SurfaceID:     placement.SurfaceID,
			WorkspaceID:   placement.WorkspaceID,
			ZoneID:        placement.ZoneID,
			Geometry:      cloneGeometry(&placement.Geometry),
			WaitTimeoutMs: 1000,
		}
		guard := func(tracked TrackedSurface) bool {
			return isAutoTileSurface(tracked)
		}
		if _, err := bridge.placeSurfaceChecked(request, "layout.auto_tile", placement.Geometry, placement.ZoneID, SurfaceLayoutRoleTiled, guard, false); err != nil {
			class, _ := classifyError(err)
			switch class {
			case ErrorSurfaceNotFound, ErrorSurfaceStale, ErrorCompositorUnavailable:
				continue
			default:
				return err
			}
		}
		applied = append(applied, placement)
	}
	if len(applied) == len(placements) && bridge.isCurrentAutoLayoutSeq(targetSeq) {
		bridge.applyAutoLayoutOrder(applied)
	}
	return nil
}

func (bridge *Bridge) isCurrentAutoLayoutSeq(targetSeq uint64) bool {
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	return bridge.autoLayoutSeq == targetSeq
}

func (bridge *Bridge) shouldApplyAutoLayoutPlacement(surfaceID string) bool {
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	tracked, ok := bridge.surfaces[surfaceID]
	return ok && isAutoTileSurface(tracked)
}

func (bridge *Bridge) applyAutoLayoutOrder(placements []autoLayoutPlacement) {
	if len(placements) == 0 {
		return
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	layout := LayoutState{
		Mode:       bridge.tiledLayoutModeLocked(),
		Settings:   bridge.layoutSettings,
		Surfaces:   make([]LayoutSurface, 0, len(placements)),
		Workspaces: []LayoutWorkspace{{ID: "workspace-1", Name: "workspace 1", Active: true}},
	}
	workspace := &layout.Workspaces[0]
	zonesByID := map[string]*LayoutZone{}
	zoneOrder := make([]string, 0, len(placements))
	placed := map[string]bool{}
	for index, placement := range placements {
		tracked, ok := bridge.surfaces[placement.SurfaceID]
		if !ok {
			continue
		}
		placed[tracked.Surface.ID] = true
		workspaceID := firstNonEmpty(placement.WorkspaceID, tracked.WorkspaceID, tracked.Surface.WorkspaceID, "workspace-1")
		if workspace.ID == "workspace-1" && workspaceID != "workspace-1" {
			workspace.ID = workspaceID
			workspace.Name = workspaceID
		}
		outputID := firstNonEmpty(tracked.OutputID, tracked.Surface.OutputID)
		if workspace.OutputID == "" {
			workspace.OutputID = outputID
		}
		zoneID := firstNonEmpty(placement.ZoneID, tracked.ZoneID, tracked.Surface.ZoneID, zoneMaster)
		if _, ok := zonesByID[zoneID]; !ok {
			zonesByID[zoneID] = &LayoutZone{ID: zoneID, Name: zoneID, Kind: "work"}
			zoneOrder = append(zoneOrder, zoneID)
		}
		zonesByID[zoneID].SurfaceIDs = append(zonesByID[zoneID].SurfaceIDs, tracked.Surface.ID)
		workspace.SurfaceOrder = append(workspace.SurfaceOrder, tracked.Surface.ID)
		label := tracked.Surface.Label
		if label == "" {
			label = fmt.Sprintf("%d", index+1)
		}
		layoutMode := bridge.tiledLayoutModeLocked()
		layout.Surfaces = append(layout.Surfaces, LayoutSurface{
			SurfaceID:     tracked.Surface.ID,
			Label:         label,
			AppID:         tracked.Surface.AppID,
			Title:         tracked.Surface.Title,
			Role:          tracked.Surface.Role,
			OutputID:      outputID,
			WorkspaceID:   workspaceID,
			ZoneID:        zoneID,
			Mode:          layoutMode,
			Participation: SurfaceLayoutRoleTiled,
			Floating:      false,
			Focused:       bridge.layoutFocusedLocked(tracked),
			Visible:       tracked.Visible,
			Geometry:      firstGeometry(tracked),
			Order:         index,
		})
		tracked.LayoutMode = string(layoutMode)
		tracked.LayoutRole = string(SurfaceLayoutRoleTiled)
		tracked.Surface.LayoutMode = tracked.LayoutMode
		tracked.Surface.LayoutRole = tracked.LayoutRole
		bridge.surfaces[tracked.Surface.ID] = tracked
	}
	remaining := make([]TrackedSurface, 0, len(bridge.surfaces))
	for _, tracked := range bridge.surfaces {
		if placed[tracked.Surface.ID] || tracked.Surface.SurfaceKind == SurfaceKindLayer || !tracked.Visible {
			continue
		}
		remaining = append(remaining, tracked)
	}
	sort.Slice(remaining, func(i, j int) bool { return remaining[i].Surface.ID < remaining[j].Surface.ID })
	for _, tracked := range remaining {
		workspaceID := firstNonEmpty(tracked.WorkspaceID, tracked.Surface.WorkspaceID, "workspace-1")
		outputID := firstNonEmpty(tracked.OutputID, tracked.Surface.OutputID)
		if workspace.OutputID == "" {
			workspace.OutputID = outputID
		}
		zoneID := firstNonEmpty(tracked.ZoneID, tracked.Surface.ZoneID, zoneTransient)
		kind := "floating"
		if SurfaceLayoutRole(tracked.LayoutRole) == SurfaceLayoutRoleTiled {
			kind = "work"
		}
		if _, ok := zonesByID[zoneID]; !ok {
			zonesByID[zoneID] = &LayoutZone{ID: zoneID, Name: zoneID, Kind: kind}
			zoneOrder = append(zoneOrder, zoneID)
		}
		zonesByID[zoneID].SurfaceIDs = append(zonesByID[zoneID].SurfaceIDs, tracked.Surface.ID)
		workspace.SurfaceOrder = append(workspace.SurfaceOrder, tracked.Surface.ID)
		label := tracked.Surface.Label
		if label == "" {
			label = fmt.Sprintf("%d", len(layout.Surfaces)+1)
		}
		role := SurfaceLayoutRole(tracked.LayoutRole)
		if role == "" {
			role = SurfaceLayoutRoleFloating
		}
		mode := LayoutMode(tracked.LayoutMode)
		if mode == "" || !validLayoutMode(mode) {
			mode = LayoutModeFreeform
		}
		layout.Surfaces = append(layout.Surfaces, LayoutSurface{
			SurfaceID:     tracked.Surface.ID,
			Label:         label,
			AppID:         tracked.Surface.AppID,
			Title:         tracked.Surface.Title,
			Role:          tracked.Surface.Role,
			OutputID:      outputID,
			WorkspaceID:   workspaceID,
			ZoneID:        zoneID,
			Mode:          mode,
			Participation: role,
			Floating:      role == SurfaceLayoutRoleFloating,
			Focused:       bridge.layoutFocusedLocked(tracked),
			Visible:       tracked.Visible,
			Geometry:      firstGeometry(tracked),
			Order:         len(layout.Surfaces),
		})
	}
	for _, zoneID := range zoneOrder {
		workspace.Zones = append(workspace.Zones, *zonesByID[zoneID])
	}
	bridge.layoutSeq++
	layout.Revision = bridge.layoutSeq
	bridge.backendLayout = cloneLayoutStatePtr(layout)
}

func (bridge *Bridge) autoLayoutPlan() []autoLayoutPlacement {
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	if bridge.plugin == nil || bridge.layoutMode == LayoutModeFreeform {
		return nil
	}
	surfaces := make([]TrackedSurface, 0, len(bridge.surfaces))
	for _, surface := range bridge.surfaces {
		if !isAutoTileSurface(surface) {
			continue
		}
		surfaces = append(surfaces, surface)
	}
	if len(surfaces) == 0 {
		return nil
	}
	sort.Slice(surfaces, func(i, j int) bool {
		promoted := bridge.promotedSurfaceID
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

	output := bridge.outputForAutoLayoutLocked(surfaces)
	if output.Name == "" {
		return nil
	}
	settings := bridge.layoutSettings
	width := firstPositive(output.PhysicalWidth, output.Width)
	height := firstPositive(output.PhysicalHeight, output.Height)
	if width <= 0 || height <= 0 {
		return nil
	}
	height -= bridge.reservedBottomHeightLocked(output.Name, width)
	if height <= 0 {
		return nil
	}
	outerH := settings.Gaps.OuterHorizontal
	outerV := settings.Gaps.OuterVertical
	if settings.SmartGaps && len(surfaces) == 1 {
		outerH = 0
		outerV = 0
	}
	width = maxInt(1, width-outerH*2)
	height = maxInt(1, height-outerV*2)

	placements := make([]autoLayoutPlacement, 0, len(surfaces))
	area := SurfaceGeometry{X: output.PhysicalX + outerH, Y: output.PhysicalY + outerV, Width: width, Height: height}
	if len(surfaces) == 1 {
		surface := surfaces[0]
		placements = append(placements, autoLayoutPlacement{
			SurfaceID:   surface.Surface.ID,
			WorkspaceID: firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, "workspace-1"),
			ZoneID:      zoneMaster,
			Geometry:    area,
		})
		return placements
	}

	nmaster := settings.MasterCount
	if nmaster <= 0 {
		nmaster = 1
	}
	if nmaster > len(surfaces) {
		nmaster = len(surfaces)
	}
	stackCount := len(surfaces) - nmaster
	innerH := 0
	if stackCount > 0 {
		innerH = minInt(settings.Gaps.InnerHorizontal, area.Width-1)
	}
	masterWidth := area.Width
	if stackCount > 0 {
		masterWidth = int(float64(area.Width-innerH) * settings.MasterRatio)
	}
	if masterWidth <= 0 {
		masterWidth = 1
	}
	stackWidth := area.Width - masterWidth - innerH
	if stackWidth <= 0 {
		stackWidth = 1
	}
	masterRects := verticalSlices(SurfaceGeometry{X: area.X, Y: area.Y, Width: masterWidth, Height: area.Height}, nmaster, settings.Gaps.InnerVertical)
	stackRects := verticalSlices(SurfaceGeometry{X: area.X + masterWidth + innerH, Y: area.Y, Width: stackWidth, Height: area.Height}, stackCount, settings.Gaps.InnerVertical)
	for index, surface := range surfaces {
		workspaceID := firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, "workspace-1")
		if index < nmaster {
			placements = append(placements, autoLayoutPlacement{
				SurfaceID:   surface.Surface.ID,
				WorkspaceID: workspaceID,
				ZoneID:      zoneMaster,
				Geometry:    masterRects[index],
			})
			continue
		}
		stackIndex := index - nmaster
		placements = append(placements, autoLayoutPlacement{
			SurfaceID:   surface.Surface.ID,
			WorkspaceID: workspaceID,
			ZoneID:      zoneStack,
			Geometry:    stackRects[stackIndex],
		})
	}
	return placements
}

func verticalSlices(area SurfaceGeometry, count int, gap int) []SurfaceGeometry {
	if count <= 0 {
		return nil
	}
	gap = minInt(gap, maxInt(0, area.Height-1))
	totalGap := gap * (count - 1)
	if totalGap >= area.Height {
		totalGap = 0
		gap = 0
	}
	availableHeight := area.Height - totalGap
	sliceHeight := maxInt(1, availableHeight/count)
	rects := make([]SurfaceGeometry, 0, count)
	y := area.Y
	for index := 0; index < count; index++ {
		height := sliceHeight
		if index == count-1 {
			height = area.Y + area.Height - y
		}
		rects = append(rects, SurfaceGeometry{X: area.X, Y: y, Width: area.Width, Height: maxInt(1, height)})
		y += height + gap
	}
	return rects
}

func (bridge *Bridge) outputForAutoLayoutLocked(surfaces []TrackedSurface) LogicalOutput {
	outputName := ""
	for _, surface := range surfaces {
		outputName = firstNonEmpty(surface.OutputID, surface.Surface.OutputID)
		if outputName != "" {
			break
		}
	}
	outputs := bridge.outputsLocked()
	if outputName != "" {
		if output := outputs[outputName]; output.Name != "" {
			return output
		}
	}
	for _, output := range outputs {
		return output
	}
	return LogicalOutput{}
}

func (bridge *Bridge) tiledLayoutModeLocked() LayoutMode {
	if bridge.layoutMode == "" || bridge.layoutMode == LayoutModeFreeform || !validLayoutMode(bridge.layoutMode) {
		return LayoutModeZones
	}
	return bridge.layoutMode
}

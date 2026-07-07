package compositorbridge

import "strings"

const (
	zoneChrome    = "chrome"
	zoneMaster    = "master"
	zoneStack     = "stack"
	zoneTransient = "transient"
)

func (bridge *Bridge) applyLifecycleClassificationLocked(surface *TrackedSurface) {
	if surface.Surface.SurfaceKind == SurfaceKindLayer {
		surface.ZoneID = firstNonEmpty(surface.ZoneID, zoneChrome)
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleTransient)
		applySurfaceLayoutFields(surface)
		return
	}
	if isExplicitFloatingSurface(*surface) {
		surface.ZoneID = firstNonEmpty(surface.ZoneID, zoneTransient)
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleFloating)
		applySurfaceLayoutFields(surface)
		return
	}
	if isShellManagedSurface(*surface) || isTransientSurfaceRole(firstNonEmpty(surface.Surface.Role, surface.LayoutRole)) {
		surface.ZoneID = zoneTransient
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleTransient)
		applySurfaceLayoutFields(surface)
		return
	}
	if bridge.layoutMode == LayoutModeFreeform {
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleFloating)
		applySurfaceLayoutFields(surface)
		return
	}
	surface.ZoneID = firstNonEmpty(surface.ZoneID, zoneMaster)
	surface.LayoutMode = string(bridge.tiledLayoutModeLocked())
	surface.LayoutRole = string(SurfaceLayoutRoleTiled)
	applySurfaceLayoutFields(surface)
}

func isAutoTileSurface(surface TrackedSurface) bool {
	if surface.Surface.SurfaceKind == SurfaceKindLayer || !surface.Visible {
		return false
	}
	if isShellManagedSurface(surface) || isTransientSurfaceRole(firstNonEmpty(surface.Surface.Role, surface.LayoutRole)) {
		return false
	}
	role := SurfaceLayoutRole(surface.LayoutRole)
	return role != SurfaceLayoutRoleFloating && role != SurfaceLayoutRoleTransient
}

func isExplicitFloatingSurface(surface TrackedSurface) bool {
	return SurfaceLayoutRole(surface.LayoutRole) == SurfaceLayoutRoleFloating &&
		LayoutMode(surface.LayoutMode) == LayoutModeFreeform &&
		firstNonEmpty(surface.ZoneID, surface.Surface.ZoneID) == zoneTransient
}

func isShellManagedSurface(surface TrackedSurface) bool {
	appID := strings.TrimSpace(firstNonEmpty(surface.Surface.AppID, surface.Surface.Role))
	switch appID {
	case "io.agorade.ShellLauncher",
		"io.agorade.ShellStatus",
		"io.agorade.ShellPanel",
		"io.agorade.ShellBackground",
		"io.agorade.ShellOverlay":
		return true
	default:
		return strings.HasPrefix(appID, "io.agorade.Shell")
	}
}

func isTransientSurfaceRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return false
	}
	for _, marker := range []string{"dialog", "modal", "popup", "popover", "menu", "tooltip", "transient", "unmanaged"} {
		if strings.Contains(role, marker) {
			return true
		}
	}
	return false
}

func applySurfaceLayoutFields(surface *TrackedSurface) {
	surface.Surface.WorkspaceID = surface.WorkspaceID
	surface.Surface.ZoneID = surface.ZoneID
	surface.Surface.LayoutMode = surface.LayoutMode
	surface.Surface.LayoutRole = surface.LayoutRole
}

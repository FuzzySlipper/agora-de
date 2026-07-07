package compositorbridge

import "strings"

const (
	zoneChrome    = "chrome"
	zoneMaster    = "master"
	zoneStack     = "stack"
	zoneTransient = "transient"
)

func (bridge *Bridge) applyLifecycleClassificationLocked(surface *TrackedSurface) {
	surface.ParentSurfaceID = firstNonEmpty(surface.ParentSurfaceID, surface.Surface.ParentSurfaceID)
	if surface.Surface.SurfaceKind == SurfaceKindLayer {
		surface.ZoneID = firstNonEmpty(surface.ZoneID, zoneChrome)
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleTransient)
		setSurfacePolicy(surface, SurfacePolicyClassShellChrome, "layer-shell chrome surface")
		applySurfaceLayoutFields(surface)
		return
	}
	if isExplicitFloatingSurface(*surface) {
		surface.ZoneID = firstNonEmpty(surface.ZoneID, zoneTransient)
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleFloating)
		setSurfacePolicy(surface, SurfacePolicyClassFloatingOverride, "explicit floating override")
		applySurfaceLayoutFields(surface)
		return
	}
	if isShellManagedSurface(*surface) {
		surface.ZoneID = zoneTransient
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleTransient)
		setSurfacePolicy(surface, SurfacePolicyClassTransient, "shell-managed transient surface")
		applySurfaceLayoutFields(surface)
		return
	}
	if isTransientSurfaceRole(firstNonEmpty(surface.Surface.Role, surface.LayoutRole)) {
		surface.ZoneID = zoneTransient
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleTransient)
		class, reason := transientSurfacePolicy(firstNonEmpty(surface.Surface.Role, surface.LayoutRole), surface.ParentSurfaceID)
		setSurfacePolicy(surface, class, reason)
		applySurfaceLayoutFields(surface)
		return
	}
	if bridge.layoutMode == LayoutModeFreeform {
		surface.LayoutMode = string(LayoutModeFreeform)
		surface.LayoutRole = string(SurfaceLayoutRoleFloating)
		setSurfacePolicy(surface, SurfacePolicyClassWork, "normal work surface in freeform layout mode")
		applySurfaceLayoutFields(surface)
		return
	}
	surface.ZoneID = firstNonEmpty(surface.ZoneID, zoneMaster)
	surface.LayoutMode = string(bridge.tiledLayoutModeLocked())
	surface.LayoutRole = string(SurfaceLayoutRoleTiled)
	setSurfacePolicy(surface, SurfacePolicyClassWork, "normal work surface")
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

func transientSurfacePolicy(role string, parentSurfaceID string) (SurfacePolicyClass, string) {
	role = strings.ToLower(strings.TrimSpace(role))
	if strings.Contains(role, "unmanaged") {
		return SurfacePolicyClassTransient, "unmanaged helper surface"
	}
	if parentSurfaceID == "" {
		return SurfacePolicyClassNoParent, "transient surface has no parent surface"
	}
	return SurfacePolicyClassTransient, "transient surface follows parent " + parentSurfaceID
}

func setSurfacePolicy(surface *TrackedSurface, class SurfacePolicyClass, reason string) {
	surface.PolicyClass = class
	surface.PolicyReason = reason
	surface.Surface.PolicyClass = class
	surface.Surface.PolicyReason = reason
}

func defaultLayoutSurfacePolicy(surface LayoutSurface) (SurfacePolicyClass, string) {
	switch {
	case surface.Participation == SurfaceLayoutRoleTransient:
		if isTransientSurfaceRole(surface.Role) && surface.ParentSurfaceID == "" {
			return SurfacePolicyClassNoParent, "transient surface has no parent surface"
		}
		return SurfacePolicyClassTransient, "transient surface"
	case surface.Participation == SurfaceLayoutRoleFloating:
		return SurfacePolicyClassFloatingOverride, "floating layout surface"
	default:
		return SurfacePolicyClassWork, "normal work surface"
	}
}

func applySurfaceLayoutFields(surface *TrackedSurface) {
	surface.Surface.ParentSurfaceID = surface.ParentSurfaceID
	surface.Surface.WorkspaceID = surface.WorkspaceID
	surface.Surface.ZoneID = surface.ZoneID
	surface.Surface.LayoutMode = surface.LayoutMode
	surface.Surface.LayoutRole = surface.LayoutRole
	surface.Surface.PolicyClass = surface.PolicyClass
	surface.Surface.PolicyReason = surface.PolicyReason
}

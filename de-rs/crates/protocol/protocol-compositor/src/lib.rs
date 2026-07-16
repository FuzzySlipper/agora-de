use de_ids::SurfaceId;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SurfaceEventKind {
    Mapped,
    Unmapped,
    Focused,
    InputDenied,
}

impl SurfaceEventKind {
    pub const ALL: [SurfaceEventKind; 4] = [
        SurfaceEventKind::Mapped,
        SurfaceEventKind::Unmapped,
        SurfaceEventKind::Focused,
        SurfaceEventKind::InputDenied,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            SurfaceEventKind::Mapped => "mapped",
            SurfaceEventKind::Unmapped => "unmapped",
            SurfaceEventKind::Focused => "focused",
            SurfaceEventKind::InputDenied => "input_denied",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SurfaceEvent {
    pub surface_id: SurfaceId,
    pub kind: SurfaceEventKind,
    pub owner_uid: u32,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LayoutMode {
    Freeform,
    Zones,
    Columns,
}

impl LayoutMode {
    pub const ALL: [LayoutMode; 3] = [LayoutMode::Freeform, LayoutMode::Zones, LayoutMode::Columns];

    pub fn wire_name(&self) -> &'static str {
        match self {
            LayoutMode::Freeform => "freeform",
            LayoutMode::Zones => "zones",
            LayoutMode::Columns => "columns",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SurfaceLayoutParticipation {
    Tiled,
    Floating,
    Transient,
}

impl SurfaceLayoutParticipation {
    pub const ALL: [SurfaceLayoutParticipation; 3] = [
        SurfaceLayoutParticipation::Tiled,
        SurfaceLayoutParticipation::Floating,
        SurfaceLayoutParticipation::Transient,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            SurfaceLayoutParticipation::Tiled => "tiled",
            SurfaceLayoutParticipation::Floating => "floating",
            SurfaceLayoutParticipation::Transient => "transient",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SurfacePolicyClass {
    Work,
    Transient,
    FloatingOverride,
    ShellChrome,
    Stale,
    Unsupported,
    NoParent,
    BackendLimited,
}

impl SurfacePolicyClass {
    pub const ALL: [SurfacePolicyClass; 8] = [
        SurfacePolicyClass::Work,
        SurfacePolicyClass::Transient,
        SurfacePolicyClass::FloatingOverride,
        SurfacePolicyClass::ShellChrome,
        SurfacePolicyClass::Stale,
        SurfacePolicyClass::Unsupported,
        SurfacePolicyClass::NoParent,
        SurfacePolicyClass::BackendLimited,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            SurfacePolicyClass::Work => "work",
            SurfacePolicyClass::Transient => "transient",
            SurfacePolicyClass::FloatingOverride => "floating_override",
            SurfacePolicyClass::ShellChrome => "shell_chrome",
            SurfacePolicyClass::Stale => "stale",
            SurfacePolicyClass::Unsupported => "unsupported",
            SurfacePolicyClass::NoParent => "no_parent",
            SurfacePolicyClass::BackendLimited => "backend_limited",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LayoutActionKind {
    GetLayout,
    SetLayoutMode,
    MoveResizeSurface,
    TileSurface,
    SetSurfaceFloating,
    AssignSurfaceZone,
    MaximizeSurface,
    MinimizeSurface,
    FullscreenSurface,
    ActivateWorkspace,
    MoveDirection,
    SwapMaster,
}

/// Cardinal direction for `LayoutActionKind::MoveDirection`. A surface moves one
/// step in the given direction within the active layout order. Left/Right cross
/// the master/stack column boundary (master-stack); Up/Down reorder within the
/// current column. The runtime planner (Go) interprets this against the active
/// rule + master count; the contract model treats Left as "toward master"
/// (index 0) and Right as "toward stack" (index 1).
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum MoveDirection {
    Left,
    Right,
    Up,
    Down,
}

impl MoveDirection {
    pub const ALL: [MoveDirection; 4] = [
        MoveDirection::Left,
        MoveDirection::Right,
        MoveDirection::Up,
        MoveDirection::Down,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            MoveDirection::Left => "left",
            MoveDirection::Right => "right",
            MoveDirection::Up => "up",
            MoveDirection::Down => "down",
        }
    }
}

impl LayoutActionKind {
    pub const ALL: [LayoutActionKind; 12] = [
        LayoutActionKind::GetLayout,
        LayoutActionKind::SetLayoutMode,
        LayoutActionKind::MoveResizeSurface,
        LayoutActionKind::TileSurface,
        LayoutActionKind::SetSurfaceFloating,
        LayoutActionKind::AssignSurfaceZone,
        LayoutActionKind::MaximizeSurface,
        LayoutActionKind::MinimizeSurface,
        LayoutActionKind::FullscreenSurface,
        LayoutActionKind::ActivateWorkspace,
        LayoutActionKind::MoveDirection,
        LayoutActionKind::SwapMaster,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            LayoutActionKind::GetLayout => "layout.get",
            LayoutActionKind::SetLayoutMode => "layout.set_mode",
            LayoutActionKind::MoveResizeSurface => "surface.move_resize",
            LayoutActionKind::TileSurface => "surface.tile",
            LayoutActionKind::SetSurfaceFloating => "surface.set_floating",
            LayoutActionKind::AssignSurfaceZone => "surface.assign_zone",
            LayoutActionKind::MaximizeSurface => "surface.maximize",
            LayoutActionKind::MinimizeSurface => "surface.minimize",
            LayoutActionKind::FullscreenSurface => "surface.fullscreen",
            LayoutActionKind::ActivateWorkspace => "workspace.activate",
            LayoutActionKind::MoveDirection => "surface.move",
            LayoutActionKind::SwapMaster => "surface.swap_master",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{
        LayoutActionKind, LayoutMode, MoveDirection, SurfaceEventKind,
        SurfaceLayoutParticipation, SurfacePolicyClass,
    };

    #[test]
    fn surface_event_wire_names_are_stable() {
        let names: Vec<&str> = SurfaceEventKind::ALL
            .iter()
            .map(SurfaceEventKind::wire_name)
            .collect();
        assert_eq!(names, vec!["mapped", "unmapped", "focused", "input_denied"]);
    }

    #[test]
    fn layout_wire_names_are_stable() {
        let modes: Vec<&str> = LayoutMode::ALL.iter().map(LayoutMode::wire_name).collect();
        assert_eq!(modes, vec!["freeform", "zones", "columns"]);

        let participation: Vec<&str> = SurfaceLayoutParticipation::ALL
            .iter()
            .map(SurfaceLayoutParticipation::wire_name)
            .collect();
        assert_eq!(participation, vec!["tiled", "floating", "transient"]);

        let policy_classes: Vec<&str> = SurfacePolicyClass::ALL
            .iter()
            .map(SurfacePolicyClass::wire_name)
            .collect();
        assert_eq!(
            policy_classes,
            vec![
                "work",
                "transient",
                "floating_override",
                "shell_chrome",
                "stale",
                "unsupported",
                "no_parent",
                "backend_limited",
            ]
        );

        let actions: Vec<&str> = LayoutActionKind::ALL
            .iter()
            .map(LayoutActionKind::wire_name)
            .collect();
        assert_eq!(
            actions,
            vec![
                "layout.get",
                "layout.set_mode",
                "surface.move_resize",
                "surface.tile",
                "surface.set_floating",
                "surface.assign_zone",
                "surface.maximize",
                "surface.minimize",
                "surface.fullscreen",
                "workspace.activate",
                "surface.move",
                "surface.swap_master",
            ]
        );

        let directions: Vec<&str> = MoveDirection::ALL
            .iter()
            .map(MoveDirection::wire_name)
            .collect();
        assert_eq!(directions, vec!["left", "right", "up", "down"]);
    }
}

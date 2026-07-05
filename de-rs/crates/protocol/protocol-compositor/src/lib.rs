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
    pub const ALL: [LayoutMode; 3] = [
        LayoutMode::Freeform,
        LayoutMode::Zones,
        LayoutMode::Columns,
    ];

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
}

impl LayoutActionKind {
    pub const ALL: [LayoutActionKind; 10] = [
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
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{LayoutActionKind, LayoutMode, SurfaceEventKind, SurfaceLayoutParticipation};

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
            ]
        );
    }
}

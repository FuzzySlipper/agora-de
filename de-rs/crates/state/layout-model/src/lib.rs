use de_ids::SurfaceId;
use protocol_compositor::{LayoutActionKind, LayoutMode, SurfaceLayoutParticipation};

mod planner;
pub use planner::{
    LayoutGaps, LayoutPlan, LayoutPlanError, LayoutRule, PlannedSurface, PlannerInput,
    PlannerSettings, PlannerSurface, ReservedChrome,
};

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ChromeSurfaceKind {
    Panel,
    Dock,
    Overlay,
    Background,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Geometry {
    pub x: i32,
    pub y: i32,
    pub width: u32,
    pub height: u32,
}

impl Geometry {
    pub fn new(x: i32, y: i32, width: u32, height: u32) -> Self {
        Self {
            x,
            y,
            width,
            height,
        }
    }

    pub fn is_empty(&self) -> bool {
        self.width == 0 || self.height == 0
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LayoutSurface {
    pub surface_id: SurfaceId,
    pub workspace_id: String,
    pub zone_id: String,
    pub geometry: Geometry,
    pub participation: SurfaceLayoutParticipation,
    pub focused: bool,
    pub visible: bool,
    pub stale: bool,
    pub order: usize,
}

impl LayoutSurface {
    pub fn tiled(surface_id: SurfaceId, zone_id: impl Into<String>, geometry: Geometry) -> Self {
        Self {
            surface_id,
            workspace_id: "workspace-1".to_string(),
            zone_id: zone_id.into(),
            geometry,
            participation: SurfaceLayoutParticipation::Tiled,
            focused: false,
            visible: true,
            stale: false,
            order: 0,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LayoutWorkspace {
    pub id: String,
    pub active: bool,
    pub surface_order: Vec<SurfaceId>,
    pub focus_order: Vec<SurfaceId>,
}

impl LayoutWorkspace {
    pub fn primary() -> Self {
        Self {
            id: "workspace-1".to_string(),
            active: true,
            surface_order: Vec::new(),
            focus_order: Vec::new(),
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LayoutState {
    pub mode: LayoutMode,
    pub revision: u64,
    pub surfaces: Vec<LayoutSurface>,
    pub workspaces: Vec<LayoutWorkspace>,
}

impl LayoutState {
    pub fn new(mode: LayoutMode) -> Self {
        Self {
            mode,
            revision: 0,
            surfaces: Vec::new(),
            workspaces: vec![LayoutWorkspace::primary()],
        }
    }

    pub fn add_surface(&mut self, mut surface: LayoutSurface) {
        surface.order = self.surfaces.len();
        let surface_id = surface.surface_id.clone();
        let workspace_id = surface.workspace_id.clone();
        self.surfaces.push(surface);
        self.ensure_workspace_contains(&workspace_id, surface_id);
        self.bump_revision();
    }

    pub fn apply(&mut self, command: LayoutCommand) -> LayoutCommandResult {
        match command.kind {
            LayoutActionKind::AssignSurfaceZone | LayoutActionKind::TileSurface => {
                self.apply_zone_command(command)
            }
            LayoutActionKind::MoveResizeSurface => self.apply_move_resize(command),
            LayoutActionKind::SetSurfaceFloating => self.apply_floating(command),
            LayoutActionKind::SetLayoutMode => self.apply_set_mode(command),
            LayoutActionKind::ActivateWorkspace => self.apply_workspace_activation(command),
            _ => LayoutCommandResult::unsupported(command.kind),
        }
    }

    pub fn focus_surface(&mut self, surface_id: &SurfaceId) -> LayoutCommandResult {
        let Some(index) = self.surface_index(surface_id) else {
            return LayoutCommandResult::missing(LayoutActionKind::GetLayout);
        };
        if self.surfaces[index].stale || !self.surfaces[index].visible {
            return LayoutCommandResult::stale(LayoutActionKind::GetLayout);
        }

        for surface in &mut self.surfaces {
            surface.focused = surface.surface_id == *surface_id;
        }
        let workspace_id = self.surfaces[index].workspace_id.clone();
        let workspace = self.ensure_workspace(&workspace_id);
        workspace.focus_order.retain(|id| id != surface_id);
        workspace.focus_order.push(surface_id.clone());
        self.bump_revision();
        LayoutCommandResult::accepted(LayoutActionKind::GetLayout, self.revision)
    }

    pub fn mark_unmapped(&mut self, surface_id: &SurfaceId) -> LayoutCommandResult {
        let Some(index) = self.surface_index(surface_id) else {
            return LayoutCommandResult::missing(LayoutActionKind::GetLayout);
        };
        self.surfaces[index].stale = true;
        self.surfaces[index].visible = false;
        for workspace in &mut self.workspaces {
            workspace.surface_order.retain(|id| id != surface_id);
            workspace.focus_order.retain(|id| id != surface_id);
        }
        self.bump_revision();
        LayoutCommandResult::accepted(LayoutActionKind::GetLayout, self.revision)
    }

    fn apply_zone_command(&mut self, command: LayoutCommand) -> LayoutCommandResult {
        let action = command.kind;
        let Some(zone_id) = command.zone_id else {
            return LayoutCommandResult::invalid(action, "zone_id is required");
        };
        let Some(geometry) = command.geometry else {
            return LayoutCommandResult::unsupported(action);
        };
        if geometry.is_empty() {
            return LayoutCommandResult::invalid(
                action,
                "geometry width and height must be positive",
            );
        }
        self.update_surface(command.surface_id, action, |surface| {
            surface.zone_id = zone_id;
            surface.geometry = geometry;
            surface.participation = SurfaceLayoutParticipation::Tiled;
        })
    }

    fn apply_move_resize(&mut self, command: LayoutCommand) -> LayoutCommandResult {
        let action = command.kind;
        let Some(geometry) = command.geometry else {
            return LayoutCommandResult::invalid(action, "geometry is required");
        };
        if geometry.is_empty() {
            return LayoutCommandResult::invalid(
                action,
                "geometry width and height must be positive",
            );
        }
        self.update_surface(command.surface_id, action, |surface| {
            surface.geometry = geometry;
        })
    }

    fn apply_floating(&mut self, command: LayoutCommand) -> LayoutCommandResult {
        let action = command.kind;
        self.update_surface(command.surface_id, action, |surface| {
            surface.participation = SurfaceLayoutParticipation::Floating;
        })
    }

    fn apply_set_mode(&mut self, command: LayoutCommand) -> LayoutCommandResult {
        let Some(mode) = command.mode else {
            return LayoutCommandResult::invalid(command.kind, "mode is required");
        };
        self.mode = mode;
        self.bump_revision();
        LayoutCommandResult::accepted(command.kind, self.revision)
    }

    fn apply_workspace_activation(&mut self, command: LayoutCommand) -> LayoutCommandResult {
        let Some(workspace_id) = command.workspace_id else {
            return LayoutCommandResult::invalid(command.kind, "workspace_id is required");
        };
        if !self
            .workspaces
            .iter()
            .any(|workspace| workspace.id == workspace_id)
        {
            return LayoutCommandResult::missing(command.kind);
        }
        for workspace in &mut self.workspaces {
            workspace.active = workspace.id == workspace_id;
        }
        self.bump_revision();
        LayoutCommandResult::accepted(command.kind, self.revision)
    }

    fn update_surface(
        &mut self,
        surface_id: SurfaceId,
        action: LayoutActionKind,
        update: impl FnOnce(&mut LayoutSurface),
    ) -> LayoutCommandResult {
        let Some(index) = self.surface_index(&surface_id) else {
            return LayoutCommandResult::missing(action);
        };
        if self.surfaces[index].stale || !self.surfaces[index].visible {
            return LayoutCommandResult::stale(action);
        }
        update(&mut self.surfaces[index]);
        self.mode = LayoutMode::Zones;
        self.rebuild_orders();
        self.bump_revision();
        LayoutCommandResult::accepted(action, self.revision)
    }

    fn surface_index(&self, surface_id: &SurfaceId) -> Option<usize> {
        self.surfaces
            .iter()
            .position(|surface| surface.surface_id == *surface_id)
    }

    fn ensure_workspace_contains(&mut self, workspace_id: &str, surface_id: SurfaceId) {
        let workspace = self.ensure_workspace(workspace_id);
        if !workspace.surface_order.contains(&surface_id) {
            workspace.surface_order.push(surface_id.clone());
        }
        if !workspace.focus_order.contains(&surface_id) {
            workspace.focus_order.push(surface_id);
        }
    }

    fn ensure_workspace(&mut self, workspace_id: &str) -> &mut LayoutWorkspace {
        if let Some(index) = self
            .workspaces
            .iter()
            .position(|workspace| workspace.id == workspace_id)
        {
            return &mut self.workspaces[index];
        }
        self.workspaces.push(LayoutWorkspace {
            id: workspace_id.to_string(),
            active: false,
            surface_order: Vec::new(),
            focus_order: Vec::new(),
        });
        self.workspaces
            .last_mut()
            .expect("workspace was just pushed")
    }

    fn rebuild_orders(&mut self) {
        for (index, surface) in self.surfaces.iter_mut().enumerate() {
            surface.order = index;
        }
        let memberships: Vec<(String, SurfaceId)> = self
            .surfaces
            .iter()
            .filter(|surface| surface.visible && !surface.stale)
            .map(|surface| (surface.workspace_id.clone(), surface.surface_id.clone()))
            .collect();
        for workspace in &mut self.workspaces {
            workspace.surface_order.clear();
            for (workspace_id, surface_id) in &memberships {
                if workspace_id == &workspace.id {
                    workspace.surface_order.push(surface_id.clone());
                }
            }
            workspace
                .focus_order
                .retain(|surface_id| workspace.surface_order.contains(surface_id));
        }
    }

    fn bump_revision(&mut self) {
        self.revision += 1;
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LayoutCommand {
    pub kind: LayoutActionKind,
    pub surface_id: SurfaceId,
    pub workspace_id: Option<String>,
    pub zone_id: Option<String>,
    pub geometry: Option<Geometry>,
    pub mode: Option<LayoutMode>,
}

impl LayoutCommand {
    pub fn assign_zone(
        surface_id: SurfaceId,
        zone_id: impl Into<String>,
        geometry: Geometry,
    ) -> Self {
        Self {
            kind: LayoutActionKind::AssignSurfaceZone,
            surface_id,
            workspace_id: None,
            zone_id: Some(zone_id.into()),
            geometry: Some(geometry),
            mode: None,
        }
    }

    pub fn unsupported_without_geometry(surface_id: SurfaceId, zone_id: impl Into<String>) -> Self {
        Self {
            kind: LayoutActionKind::AssignSurfaceZone,
            surface_id,
            workspace_id: None,
            zone_id: Some(zone_id.into()),
            geometry: None,
            mode: None,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LayoutCommandResult {
    pub action: LayoutActionKind,
    pub status: LayoutCommandStatus,
    pub error_class: Option<LayoutErrorClass>,
    pub revision: Option<u64>,
    pub message: Option<String>,
}

impl LayoutCommandResult {
    pub fn accepted(action: LayoutActionKind, revision: u64) -> Self {
        Self {
            action,
            status: LayoutCommandStatus::Accepted,
            error_class: None,
            revision: Some(revision),
            message: None,
        }
    }

    pub fn unsupported(action: LayoutActionKind) -> Self {
        Self::rejected(action, LayoutErrorClass::BackendUnsupported, None)
    }

    pub fn missing(action: LayoutActionKind) -> Self {
        Self::rejected(action, LayoutErrorClass::SurfaceNotFound, None)
    }

    pub fn stale(action: LayoutActionKind) -> Self {
        Self::rejected(action, LayoutErrorClass::SurfaceStale, None)
    }

    pub fn invalid(action: LayoutActionKind, message: impl Into<String>) -> Self {
        Self::rejected(
            action,
            LayoutErrorClass::InvalidRequest,
            Some(message.into()),
        )
    }

    fn rejected(
        action: LayoutActionKind,
        error_class: LayoutErrorClass,
        message: Option<String>,
    ) -> Self {
        Self {
            action,
            status: LayoutCommandStatus::Rejected,
            error_class: Some(error_class),
            revision: None,
            message,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LayoutCommandStatus {
    Accepted,
    Rejected,
}

impl LayoutCommandStatus {
    pub fn wire_name(&self) -> &'static str {
        match self {
            LayoutCommandStatus::Accepted => "accepted",
            LayoutCommandStatus::Rejected => "rejected",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LayoutErrorClass {
    InvalidRequest,
    SurfaceNotFound,
    SurfaceStale,
    BackendUnsupported,
}

impl LayoutErrorClass {
    pub const ALL: [LayoutErrorClass; 4] = [
        LayoutErrorClass::InvalidRequest,
        LayoutErrorClass::SurfaceNotFound,
        LayoutErrorClass::SurfaceStale,
        LayoutErrorClass::BackendUnsupported,
    ];

    pub fn wire_name(&self) -> &'static str {
        match self {
            LayoutErrorClass::InvalidRequest => "invalid_request",
            LayoutErrorClass::SurfaceNotFound => "surface_not_found",
            LayoutErrorClass::SurfaceStale => "surface_stale",
            LayoutErrorClass::BackendUnsupported => "backend_unsupported",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{
        Geometry, LayoutCommand, LayoutCommandStatus, LayoutErrorClass, LayoutGaps, LayoutPlan,
        LayoutPlanError, LayoutRule, LayoutState, LayoutSurface, PlannerInput, PlannerSettings,
        PlannerSurface, ReservedChrome,
    };
    use de_ids::SurfaceId;
    use protocol_compositor::{LayoutActionKind, LayoutMode, SurfaceLayoutParticipation};

    #[test]
    fn zones_planner_produces_desired_rectangles_with_stable_order() {
        let mut input = PlannerInput::new(Geometry::new(0, 0, 2560, 1440), "workspace-1");
        input.revision = 41;
        input.reserved_chrome = ReservedChrome {
            top: 0,
            right: 0,
            bottom: 96,
            left: 0,
        };
        input.settings = PlannerSettings {
            gaps: LayoutGaps {
                outer_horizontal: 0,
                outer_vertical: 0,
                inner_horizontal: 0,
                inner_vertical: 0,
            },
            nmaster: 1,
            mfact: 0.5,
            smart_gaps: true,
        };
        input.surfaces = vec![
            PlannerSurface::tiled(SurfaceId::new("view-b"), 1),
            PlannerSurface::tiled(SurfaceId::new("view-a"), 0),
        ];
        input.focus_order = vec![SurfaceId::new("view-b"), SurfaceId::new("view-a")];

        let plan = LayoutPlan::plan(&input).expect("zones plan should be supported");

        assert_eq!(plan.rule, LayoutRule::Zones);
        assert_eq!(plan.mode, LayoutMode::Zones);
        assert_eq!(plan.revision, 42);
        assert_eq!(
            plan.surface_order,
            vec![SurfaceId::new("view-a"), SurfaceId::new("view-b")]
        );
        assert_eq!(
            plan.focus_order,
            vec![SurfaceId::new("view-b"), SurfaceId::new("view-a")]
        );
        assert_eq!(plan.surfaces[0].surface_id, SurfaceId::new("view-a"));
        assert_eq!(plan.surfaces[0].zone_id, "primary");
        assert_eq!(
            plan.surfaces[0].desired_geometry,
            Geometry::new(0, 0, 1280, 1344)
        );
        assert_eq!(plan.surfaces[1].surface_id, SurfaceId::new("view-b"));
        assert_eq!(plan.surfaces[1].zone_id, "secondary");
        assert_eq!(
            plan.surfaces[1].desired_geometry,
            Geometry::new(1280, 0, 1280, 1344)
        );
    }

    #[test]
    fn zones_planner_keeps_focus_order_unique_and_appends_missing_surfaces() {
        let mut input = PlannerInput::new(Geometry::new(0, 0, 1000, 800), "workspace-1");
        input.surfaces = vec![
            PlannerSurface::tiled(SurfaceId::new("view-a"), 0),
            PlannerSurface::tiled(SurfaceId::new("view-b"), 1),
        ];
        input.focus_order = vec![
            SurfaceId::new("view-missing"),
            SurfaceId::new("view-b"),
            SurfaceId::new("view-b"),
        ];

        let plan = LayoutPlan::plan(&input).expect("zones plan should be supported");

        assert_eq!(
            plan.focus_order,
            vec![SurfaceId::new("view-b"), SurfaceId::new("view-a")]
        );
    }

    #[test]
    fn unsupported_planner_rules_are_explicit() {
        let mut input = PlannerInput::new(Geometry::new(0, 0, 1000, 800), "workspace-1");
        input.rule = LayoutRule::MasterStack;
        input.surfaces = vec![PlannerSurface::tiled(SurfaceId::new("view-a"), 0)];

        let error = LayoutPlan::plan(&input).expect_err("master-stack belongs to task 4320");

        assert_eq!(
            error,
            LayoutPlanError::UnsupportedRule(LayoutRule::MasterStack)
        );
    }

    #[test]
    fn assign_zone_applies_backend_geometry_and_advances_revision() {
        let mut state = LayoutState::new(LayoutMode::Freeform);
        state.add_surface(LayoutSurface::tiled(
            SurfaceId::new("view-a"),
            "primary",
            Geometry::new(96, 66, 804, 634),
        ));
        let start_revision = state.revision;

        let result = state.apply(LayoutCommand::assign_zone(
            SurfaceId::new("view-a"),
            "secondary",
            Geometry::new(1280, 0, 1280, 1248),
        ));

        assert_eq!(result.status, LayoutCommandStatus::Accepted);
        assert_eq!(result.revision, Some(start_revision + 1));
        assert_eq!(state.mode, LayoutMode::Zones);
        assert_eq!(state.surfaces[0].zone_id, "secondary");
        assert_eq!(
            state.surfaces[0].geometry,
            Geometry::new(1280, 0, 1280, 1248)
        );
        assert_eq!(
            state.surfaces[0].participation,
            SurfaceLayoutParticipation::Tiled
        );
        assert_eq!(
            state.workspaces[0].surface_order,
            vec![SurfaceId::new("view-a")]
        );
    }

    #[test]
    fn zone_assignment_without_backend_geometry_is_unsupported() {
        let mut state = LayoutState::new(LayoutMode::Freeform);
        state.add_surface(LayoutSurface::tiled(
            SurfaceId::new("view-a"),
            "primary",
            Geometry::new(96, 66, 804, 634),
        ));

        let result = state.apply(LayoutCommand::unsupported_without_geometry(
            SurfaceId::new("view-a"),
            "secondary",
        ));

        assert_eq!(result.status, LayoutCommandStatus::Rejected);
        assert_eq!(
            result.error_class,
            Some(LayoutErrorClass::BackendUnsupported)
        );
        assert_eq!(state.surfaces[0].zone_id, "primary");
    }

    #[test]
    fn stale_and_missing_surfaces_have_stable_error_classes() {
        let mut state = LayoutState::new(LayoutMode::Freeform);
        state.add_surface(LayoutSurface::tiled(
            SurfaceId::new("view-a"),
            "primary",
            Geometry::new(0, 0, 1280, 1248),
        ));
        state.mark_unmapped(&SurfaceId::new("view-a"));

        let stale = state.apply(LayoutCommand::assign_zone(
            SurfaceId::new("view-a"),
            "secondary",
            Geometry::new(1280, 0, 1280, 1248),
        ));
        let missing = state.apply(LayoutCommand::assign_zone(
            SurfaceId::new("view-missing"),
            "secondary",
            Geometry::new(1280, 0, 1280, 1248),
        ));

        assert_eq!(stale.error_class, Some(LayoutErrorClass::SurfaceStale));
        assert_eq!(missing.error_class, Some(LayoutErrorClass::SurfaceNotFound));
    }

    #[test]
    fn focus_order_tracks_last_focused_surface() {
        let mut state = LayoutState::new(LayoutMode::Freeform);
        state.add_surface(LayoutSurface::tiled(
            SurfaceId::new("view-a"),
            "primary",
            Geometry::new(0, 0, 1280, 1248),
        ));
        state.add_surface(LayoutSurface::tiled(
            SurfaceId::new("view-b"),
            "secondary",
            Geometry::new(1280, 0, 1280, 1248),
        ));

        state.focus_surface(&SurfaceId::new("view-a"));
        state.focus_surface(&SurfaceId::new("view-b"));

        assert!(!state.surfaces[0].focused);
        assert!(state.surfaces[1].focused);
        assert_eq!(
            state.workspaces[0].focus_order,
            vec![SurfaceId::new("view-a"), SurfaceId::new("view-b")]
        );
    }

    #[test]
    fn unsupported_actions_preserve_backend_unsupported_class() {
        let mut state = LayoutState::new(LayoutMode::Freeform);
        state.add_surface(LayoutSurface::tiled(
            SurfaceId::new("view-a"),
            "primary",
            Geometry::new(0, 0, 1280, 1248),
        ));

        let result = state.apply(super::LayoutCommand {
            kind: LayoutActionKind::MaximizeSurface,
            surface_id: SurfaceId::new("view-a"),
            workspace_id: None,
            zone_id: None,
            geometry: None,
            mode: None,
        });

        assert_eq!(
            result.error_class,
            Some(LayoutErrorClass::BackendUnsupported)
        );
    }

    #[test]
    fn error_class_wire_names_are_stable() {
        let names: Vec<&str> = LayoutErrorClass::ALL
            .iter()
            .map(LayoutErrorClass::wire_name)
            .collect();
        assert_eq!(
            names,
            vec![
                "invalid_request",
                "surface_not_found",
                "surface_stale",
                "backend_unsupported",
            ]
        );
    }
}

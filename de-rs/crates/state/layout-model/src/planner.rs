use crate::Geometry;
use de_ids::SurfaceId;
use protocol_compositor::{LayoutMode, SurfaceLayoutParticipation};

#[derive(Clone, Debug, PartialEq)]
pub struct PlannerInput {
    pub rule: LayoutRule,
    pub output: Geometry,
    pub reserved_chrome: ReservedChrome,
    pub workspace_id: String,
    pub surfaces: Vec<PlannerSurface>,
    pub focus_order: Vec<SurfaceId>,
    pub settings: PlannerSettings,
    pub revision: u64,
}

impl PlannerInput {
    pub fn new(output: Geometry, workspace_id: impl Into<String>) -> Self {
        Self {
            rule: LayoutRule::Zones,
            output,
            reserved_chrome: ReservedChrome::none(),
            workspace_id: workspace_id.into(),
            surfaces: Vec::new(),
            focus_order: Vec::new(),
            settings: PlannerSettings::default(),
            revision: 0,
        }
    }

    pub fn work_area(&self) -> Geometry {
        self.reserved_chrome.apply_to(&self.output)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LayoutRule {
    Zones,
    MasterStack,
    Dwindle,
}

impl LayoutRule {
    pub fn wire_name(&self) -> &'static str {
        match self {
            LayoutRule::Zones => "zones",
            LayoutRule::MasterStack => "master_stack",
            LayoutRule::Dwindle => "dwindle",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReservedChrome {
    pub top: u32,
    pub right: u32,
    pub bottom: u32,
    pub left: u32,
}

impl ReservedChrome {
    pub fn none() -> Self {
        Self {
            top: 0,
            right: 0,
            bottom: 0,
            left: 0,
        }
    }

    pub fn apply_to(&self, geometry: &Geometry) -> Geometry {
        let horizontal = self.left.saturating_add(self.right);
        let vertical = self.top.saturating_add(self.bottom);
        Geometry {
            x: geometry.x.saturating_add(clamped_i32(self.left)),
            y: geometry.y.saturating_add(clamped_i32(self.top)),
            width: geometry.width.saturating_sub(horizontal),
            height: geometry.height.saturating_sub(vertical),
        }
    }
}

#[derive(Clone, Debug, PartialEq)]
pub struct PlannerSettings {
    pub gaps: LayoutGaps,
    pub nmaster: usize,
    pub mfact: f32,
    pub smart_gaps: bool,
}

impl Default for PlannerSettings {
    fn default() -> Self {
        Self {
            gaps: LayoutGaps::none(),
            nmaster: 1,
            mfact: 0.5,
            smart_gaps: true,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LayoutGaps {
    pub outer_horizontal: u32,
    pub outer_vertical: u32,
    pub inner_horizontal: u32,
    pub inner_vertical: u32,
}

impl LayoutGaps {
    pub fn none() -> Self {
        Self {
            outer_horizontal: 0,
            outer_vertical: 0,
            inner_horizontal: 0,
            inner_vertical: 0,
        }
    }

    pub fn apply_outer_to(
        &self,
        geometry: &Geometry,
        surface_count: usize,
        smart_gaps: bool,
    ) -> Geometry {
        if smart_gaps && surface_count <= 1 {
            return geometry.clone();
        }
        let horizontal = self.outer_horizontal.saturating_mul(2);
        let vertical = self.outer_vertical.saturating_mul(2);
        Geometry {
            x: geometry
                .x
                .saturating_add(clamped_i32(self.outer_horizontal)),
            y: geometry.y.saturating_add(clamped_i32(self.outer_vertical)),
            width: geometry.width.saturating_sub(horizontal),
            height: geometry.height.saturating_sub(vertical),
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PlannerSurface {
    pub surface_id: SurfaceId,
    pub workspace_id: String,
    pub participation: SurfaceLayoutParticipation,
    pub zone_id: Option<String>,
    pub geometry: Option<Geometry>,
    pub order: usize,
    pub focused: bool,
}

impl PlannerSurface {
    pub fn tiled(surface_id: SurfaceId, order: usize) -> Self {
        Self {
            surface_id,
            workspace_id: "workspace-1".to_string(),
            participation: SurfaceLayoutParticipation::Tiled,
            zone_id: None,
            geometry: None,
            order,
            focused: false,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LayoutPlan {
    pub rule: LayoutRule,
    pub mode: LayoutMode,
    pub revision: u64,
    pub workspace_id: String,
    pub surfaces: Vec<PlannedSurface>,
    pub surface_order: Vec<SurfaceId>,
    pub focus_order: Vec<SurfaceId>,
}

impl LayoutPlan {
    pub fn plan(input: &PlannerInput) -> Result<Self, LayoutPlanError> {
        if input.output.is_empty() {
            return Err(LayoutPlanError::InvalidInput(
                "output geometry must be non-empty",
            ));
        }
        match input.rule {
            LayoutRule::Zones => Ok(plan_zones(input)),
            LayoutRule::MasterStack | LayoutRule::Dwindle => {
                Err(LayoutPlanError::UnsupportedRule(input.rule.clone()))
            }
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PlannedSurface {
    pub surface_id: SurfaceId,
    pub workspace_id: String,
    pub zone_id: String,
    pub participation: SurfaceLayoutParticipation,
    pub desired_geometry: Geometry,
    pub order: usize,
    pub focused: bool,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LayoutPlanError {
    InvalidInput(&'static str),
    UnsupportedRule(LayoutRule),
}

fn plan_zones(input: &PlannerInput) -> LayoutPlan {
    let mut surfaces = input.surfaces.clone();
    surfaces.sort_by_key(|surface| surface.order);
    let work_area = input.work_area();
    let normal_count = surfaces
        .iter()
        .filter(|surface| surface.participation == SurfaceLayoutParticipation::Tiled)
        .count();
    let area =
        input
            .settings
            .gaps
            .apply_outer_to(&work_area, normal_count, input.settings.smart_gaps);
    let surface_count = normal_count.max(1) as u32;
    let inner_gap = if normal_count > 1 {
        input.settings.gaps.inner_horizontal
    } else {
        0
    };
    let first_width = if normal_count <= 1 {
        area.width
    } else {
        area.width.saturating_sub(inner_gap) / 2
    };
    let second_width = area
        .width
        .saturating_sub(first_width)
        .saturating_sub(inner_gap);
    let transient_height = if surface_count > 2 {
        area.height / surface_count
    } else {
        area.height
    };

    let mut planned = Vec::new();
    let mut tiled_seen = 0usize;
    for surface in surfaces {
        let (zone_id, desired_geometry, participation) = match surface.participation {
            SurfaceLayoutParticipation::Floating | SurfaceLayoutParticipation::Transient => {
                let geometry = surface.geometry.unwrap_or_else(|| {
                    Geometry::new(area.x, area.y, area.width, transient_height.max(1))
                });
                (
                    surface
                        .zone_id
                        .clone()
                        .unwrap_or_else(|| "transient".to_string()),
                    geometry,
                    surface.participation,
                )
            }
            SurfaceLayoutParticipation::Tiled => {
                let planned = if tiled_seen == 0 {
                    (
                        "primary".to_string(),
                        Geometry::new(area.x, area.y, first_width, area.height),
                    )
                } else {
                    (
                        "secondary".to_string(),
                        Geometry::new(
                            area.x
                                .saturating_add(clamped_i32(first_width.saturating_add(inner_gap))),
                            area.y,
                            second_width,
                            area.height,
                        ),
                    )
                };
                tiled_seen += 1;
                (planned.0, planned.1, SurfaceLayoutParticipation::Tiled)
            }
        };
        planned.push(PlannedSurface {
            surface_id: surface.surface_id,
            workspace_id: input.workspace_id.clone(),
            zone_id,
            participation,
            desired_geometry,
            order: planned.len(),
            focused: surface.focused,
        });
    }

    let surface_order = planned
        .iter()
        .map(|surface| surface.surface_id.clone())
        .collect();
    let focus_order = normalize_focus_order(&input.focus_order, &planned);

    LayoutPlan {
        rule: LayoutRule::Zones,
        mode: LayoutMode::Zones,
        revision: input.revision.saturating_add(1),
        workspace_id: input.workspace_id.clone(),
        surfaces: planned,
        surface_order,
        focus_order,
    }
}

fn normalize_focus_order(focus_order: &[SurfaceId], planned: &[PlannedSurface]) -> Vec<SurfaceId> {
    let mut normalized = Vec::new();
    for surface_id in focus_order {
        if planned
            .iter()
            .any(|surface| surface.surface_id == *surface_id)
            && !normalized.contains(surface_id)
        {
            normalized.push(surface_id.clone());
        }
    }
    for surface in planned {
        if !normalized.contains(&surface.surface_id) {
            normalized.push(surface.surface_id.clone());
        }
    }
    normalized
}

fn clamped_i32(value: u32) -> i32 {
    value.min(i32::MAX as u32) as i32
}

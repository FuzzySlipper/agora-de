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
        let outer_horizontal = self
            .outer_horizontal
            .min(geometry.width.saturating_sub(1) / 2);
        let outer_vertical = self
            .outer_vertical
            .min(geometry.height.saturating_sub(1) / 2);
        let horizontal = outer_horizontal.saturating_mul(2);
        let vertical = outer_vertical.saturating_mul(2);
        Geometry {
            x: geometry.x.saturating_add(clamped_i32(outer_horizontal)),
            y: geometry.y.saturating_add(clamped_i32(outer_vertical)),
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
            LayoutRule::MasterStack => Ok(plan_master_stack(input)),
            LayoutRule::Dwindle => Err(LayoutPlanError::UnsupportedRule(input.rule.clone())),
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

fn plan_master_stack(input: &PlannerInput) -> LayoutPlan {
    let mut surfaces = input.surfaces.clone();
    surfaces.sort_by_key(|surface| surface.order);
    let tiled_count = surfaces
        .iter()
        .filter(|surface| surface.participation == SurfaceLayoutParticipation::Tiled)
        .count();
    let work_area = input.work_area();
    let area =
        input
            .settings
            .gaps
            .apply_outer_to(&work_area, tiled_count, input.settings.smart_gaps);
    let nmaster = input.settings.nmaster.min(tiled_count);
    let stack_count = tiled_count.saturating_sub(nmaster);
    let inner_h = if nmaster > 0 && stack_count > 0 {
        clamp_gap(input.settings.gaps.inner_horizontal, area.width)
    } else {
        0
    };
    let available_width = area.width.saturating_sub(inner_h).max(1);
    let mfact = input.settings.mfact.clamp(0.1, 0.9);
    let master_width = if stack_count == 0 {
        area.width
    } else if nmaster == 0 {
        0
    } else {
        ((available_width as f32) * mfact).round() as u32
    }
    .min(available_width);
    let stack_width = if stack_count == 0 {
        0
    } else {
        available_width.saturating_sub(master_width).max(1)
    };
    let master_area = Geometry::new(area.x, area.y, master_width.max(1), area.height.max(1));
    let stack_area = Geometry::new(
        area.x
            .saturating_add(clamped_i32(master_width.saturating_add(inner_h))),
        area.y,
        stack_width,
        area.height.max(1),
    );
    let master_slices = vertical_slices(master_area, nmaster, input.settings.gaps.inner_vertical);
    let stack_slices = vertical_slices(stack_area, stack_count, input.settings.gaps.inner_vertical);

    let mut planned = Vec::new();
    let mut tiled_seen = 0usize;
    for surface in surfaces {
        let (zone_id, desired_geometry, participation) = match surface.participation {
            SurfaceLayoutParticipation::Floating | SurfaceLayoutParticipation::Transient => {
                let geometry = surface.geometry.unwrap_or_else(|| {
                    Geometry::new(area.x, area.y, area.width.max(1), area.height.max(1))
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
                let planned = if tiled_seen < nmaster {
                    (
                        "master".to_string(),
                        master_slices
                            .get(tiled_seen)
                            .cloned()
                            .unwrap_or_else(|| Geometry::new(area.x, area.y, 1, 1)),
                    )
                } else {
                    let stack_index = tiled_seen.saturating_sub(nmaster);
                    (
                        "stack".to_string(),
                        stack_slices
                            .get(stack_index)
                            .cloned()
                            .unwrap_or_else(|| Geometry::new(area.x, area.y, 1, 1)),
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
        rule: LayoutRule::MasterStack,
        mode: LayoutMode::Zones,
        revision: input.revision.saturating_add(1),
        workspace_id: input.workspace_id.clone(),
        surfaces: planned,
        surface_order,
        focus_order,
    }
}

fn vertical_slices(area: Geometry, count: usize, requested_gap: u32) -> Vec<Geometry> {
    if count == 0 {
        return Vec::new();
    }
    if count == 1 {
        return vec![area];
    }
    let gap = clamp_repeated_gap(requested_gap, area.height, count);
    let total_gap = gap.saturating_mul((count - 1) as u32);
    let available = area.height.saturating_sub(total_gap).max(count as u32);
    let base = available / count as u32;
    let mut remainder = available % count as u32;
    let mut y = area.y;
    let mut slices = Vec::with_capacity(count);

    for _ in 0..count {
        let extra = u32::from(remainder > 0);
        remainder = remainder.saturating_sub(extra);
        let height = base.saturating_add(extra).max(1);
        slices.push(Geometry::new(area.x, y, area.width.max(1), height));
        y = y.saturating_add(clamped_i32(height.saturating_add(gap)));
    }

    slices
}

fn clamp_gap(requested_gap: u32, extent: u32) -> u32 {
    requested_gap.min(extent.saturating_sub(1))
}

fn clamp_repeated_gap(requested_gap: u32, extent: u32, count: usize) -> u32 {
    if count <= 1 {
        return 0;
    }
    let max_gap = extent.saturating_sub(count as u32) / (count - 1) as u32;
    requested_gap.min(max_gap)
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

#[cfg(test)]
mod tests {
    use super::{
        Geometry, LayoutGaps, LayoutPlan, LayoutPlanError, LayoutRule, PlannerInput,
        PlannerSettings, PlannerSurface, ReservedChrome,
    };
    use de_ids::SurfaceId;
    use protocol_compositor::LayoutMode;

    #[test]
    fn master_stack_single_surface_uses_full_work_area_with_smartgaps() {
        let mut input = PlannerInput::new(Geometry::new(0, 0, 1000, 800), "workspace-1");
        input.rule = LayoutRule::MasterStack;
        input.settings = PlannerSettings {
            gaps: LayoutGaps {
                outer_horizontal: 40,
                outer_vertical: 40,
                inner_horizontal: 16,
                inner_vertical: 16,
            },
            nmaster: 1,
            mfact: 0.6,
            smart_gaps: true,
        };
        input.surfaces = vec![PlannerSurface::tiled(SurfaceId::new("view-a"), 0)];

        let plan = LayoutPlan::plan(&input).expect("master-stack should be supported");

        assert_eq!(plan.rule, LayoutRule::MasterStack);
        assert_eq!(plan.surfaces[0].zone_id, "master");
        assert_eq!(
            plan.surfaces[0].desired_geometry,
            Geometry::new(0, 0, 1000, 800)
        );
    }

    #[test]
    fn master_stack_two_surfaces_splits_master_and_stack() {
        let plan = sample_master_stack_plan(2, 1);

        assert_eq!(plan.mode, LayoutMode::Zones);
        assert_eq!(
            plan.surface_order,
            vec![SurfaceId::new("view-a"), SurfaceId::new("view-b")]
        );
        assert_eq!(plan.surfaces[0].zone_id, "master");
        assert_eq!(
            plan.surfaces[0].desired_geometry,
            Geometry::new(10, 20, 583, 760)
        );
        assert_eq!(plan.surfaces[1].zone_id, "stack");
        assert_eq!(
            plan.surfaces[1].desired_geometry,
            Geometry::new(601, 20, 389, 760)
        );
    }

    #[test]
    fn master_stack_three_surfaces_stacks_secondary_area() {
        let plan = sample_master_stack_plan(3, 1);

        assert_eq!(plan.surfaces[0].zone_id, "master");
        assert_eq!(
            plan.surfaces[0].desired_geometry,
            Geometry::new(10, 20, 583, 760)
        );
        assert_eq!(plan.surfaces[1].zone_id, "stack");
        assert_eq!(
            plan.surfaces[1].desired_geometry,
            Geometry::new(601, 20, 389, 378)
        );
        assert_eq!(plan.surfaces[2].zone_id, "stack");
        assert_eq!(
            plan.surfaces[2].desired_geometry,
            Geometry::new(601, 402, 389, 378)
        );
    }

    #[test]
    fn master_stack_multiple_masters_split_master_area() {
        let plan = sample_master_stack_plan(3, 2);

        assert_eq!(plan.surfaces[0].zone_id, "master");
        assert_eq!(
            plan.surfaces[0].desired_geometry,
            Geometry::new(10, 20, 583, 378)
        );
        assert_eq!(plan.surfaces[1].zone_id, "master");
        assert_eq!(
            plan.surfaces[1].desired_geometry,
            Geometry::new(10, 402, 583, 378)
        );
        assert_eq!(plan.surfaces[2].zone_id, "stack");
        assert_eq!(
            plan.surfaces[2].desired_geometry,
            Geometry::new(601, 20, 389, 760)
        );
    }

    #[test]
    fn master_stack_without_stack_uses_full_width_for_masters() {
        let plan = sample_master_stack_plan(2, 8);

        assert_eq!(plan.surfaces[0].zone_id, "master");
        assert_eq!(
            plan.surfaces[0].desired_geometry,
            Geometry::new(10, 20, 980, 378)
        );
        assert_eq!(plan.surfaces[1].zone_id, "master");
        assert_eq!(
            plan.surfaces[1].desired_geometry,
            Geometry::new(10, 402, 980, 378)
        );
    }

    #[test]
    fn master_stack_clamps_large_gaps_and_ratios() {
        let mut input = PlannerInput::new(Geometry::new(0, 0, 10, 8), "workspace-1");
        input.rule = LayoutRule::MasterStack;
        input.settings = PlannerSettings {
            gaps: LayoutGaps {
                outer_horizontal: 100,
                outer_vertical: 100,
                inner_horizontal: 100,
                inner_vertical: 100,
            },
            nmaster: 1,
            mfact: 42.0,
            smart_gaps: false,
        };
        input.surfaces = vec![
            PlannerSurface::tiled(SurfaceId::new("view-a"), 0),
            PlannerSurface::tiled(SurfaceId::new("view-b"), 1),
            PlannerSurface::tiled(SurfaceId::new("view-c"), 2),
        ];

        let plan = LayoutPlan::plan(&input).expect("master-stack should clamp tiny geometry");

        for surface in &plan.surfaces {
            assert!(surface.desired_geometry.width > 0);
            assert!(surface.desired_geometry.height > 0);
        }
    }

    #[test]
    fn dwindle_remains_explicitly_unsupported_until_task_4321() {
        let mut input = PlannerInput::new(Geometry::new(0, 0, 1000, 800), "workspace-1");
        input.rule = LayoutRule::Dwindle;
        input.surfaces = vec![PlannerSurface::tiled(SurfaceId::new("view-a"), 0)];

        let error = LayoutPlan::plan(&input).expect_err("dwindle belongs to task 4321");

        assert_eq!(error, LayoutPlanError::UnsupportedRule(LayoutRule::Dwindle));
    }

    fn sample_master_stack_plan(surface_count: usize, nmaster: usize) -> super::LayoutPlan {
        let mut input = PlannerInput::new(Geometry::new(0, 0, 1000, 800), "workspace-1");
        input.rule = LayoutRule::MasterStack;
        input.revision = 5;
        input.settings = PlannerSettings {
            gaps: LayoutGaps {
                outer_horizontal: 10,
                outer_vertical: 20,
                inner_horizontal: 8,
                inner_vertical: 4,
            },
            nmaster,
            mfact: 0.6,
            smart_gaps: false,
        };
        input.reserved_chrome = ReservedChrome::none();
        input.surfaces = (0..surface_count)
            .map(|index| {
                PlannerSurface::tiled(
                    SurfaceId::new(format!("view-{}", (b'a' + index as u8) as char)),
                    index,
                )
            })
            .collect();
        input.focus_order = input
            .surfaces
            .iter()
            .map(|surface| surface.surface_id.clone())
            .collect();
        LayoutPlan::plan(&input).expect("sample master-stack should plan")
    }
}

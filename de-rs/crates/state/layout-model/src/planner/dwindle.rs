use super::{
    clamp_gap, clamped_i32, normalize_focus_order, LayoutPlan, LayoutRule, PlannedSurface,
    PlannerInput,
};
use crate::Geometry;
use de_ids::SurfaceId;
use protocol_compositor::{LayoutMode, SurfaceLayoutParticipation};
use std::collections::HashMap;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SplitAxis {
    Horizontal,
    Vertical,
}

impl SplitAxis {
    pub fn wire_name(&self) -> &'static str {
        match self {
            SplitAxis::Horizontal => "horizontal",
            SplitAxis::Vertical => "vertical",
        }
    }
}

#[derive(Clone, Debug, PartialEq)]
pub enum DwindleNode {
    Leaf {
        surface_id: SurfaceId,
    },
    Split {
        axis: SplitAxis,
        ratio: f32,
        first: Box<DwindleNode>,
        second: Box<DwindleNode>,
    },
}

#[derive(Clone, Debug, PartialEq)]
pub struct DwindleTree {
    pub root: Option<DwindleNode>,
}

impl DwindleTree {
    pub fn empty() -> Self {
        Self { root: None }
    }

    pub fn single(surface_id: SurfaceId) -> Self {
        Self {
            root: Some(DwindleNode::Leaf { surface_id }),
        }
    }

    pub fn from_surface_order(surface_ids: &[SurfaceId], ratio: f32) -> Self {
        let Some(first) = surface_ids.first() else {
            return Self::empty();
        };
        let mut tree = Self::single(first.clone());
        let mut focused = first.clone();
        for (index, surface_id) in surface_ids.iter().enumerate().skip(1) {
            let axis = if index % 2 == 1 {
                SplitAxis::Horizontal
            } else {
                SplitAxis::Vertical
            };
            tree.insert_relative(surface_id.clone(), &focused, axis, ratio, false);
            focused = surface_id.clone();
        }
        tree
    }

    pub fn insert_relative(
        &mut self,
        surface_id: SurfaceId,
        focused_id: &SurfaceId,
        axis: SplitAxis,
        ratio: f32,
        as_first: bool,
    ) -> bool {
        if self.root.is_none() {
            self.root = Some(DwindleNode::Leaf { surface_id });
            return true;
        }
        let Some(root) = &mut self.root else {
            return false;
        };
        insert_into(
            root,
            surface_id,
            focused_id,
            axis,
            clamp_ratio(ratio),
            as_first,
        )
    }

    pub fn remove_surface(&mut self, surface_id: &SurfaceId) -> bool {
        let Some(root) = self.root.take() else {
            return false;
        };
        let (next, removed) = remove_from(root, surface_id);
        self.root = next;
        removed
    }

    pub fn assignments(&self, area: Geometry, gap_h: u32, gap_v: u32) -> Vec<DwindleAssignment> {
        let Some(root) = &self.root else {
            return Vec::new();
        };
        let mut assignments = Vec::new();
        assign_node(root, area, gap_h, gap_v, &mut assignments);
        assignments
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DwindleAssignment {
    pub surface_id: SurfaceId,
    pub geometry: Geometry,
}

pub fn plan_dwindle(input: &PlannerInput) -> LayoutPlan {
    let mut surfaces = input.surfaces.clone();
    surfaces.sort_by_key(|surface| surface.order);
    let surface_ids: Vec<SurfaceId> = surfaces
        .iter()
        .filter(|surface| surface.participation == SurfaceLayoutParticipation::Tiled)
        .map(|surface| surface.surface_id.clone())
        .collect();
    let tree = input
        .dwindle_tree
        .clone()
        .unwrap_or_else(|| DwindleTree::from_surface_order(&surface_ids, 0.5));
    let area = input.settings.gaps.apply_outer_to(
        &input.work_area(),
        surface_ids.len(),
        input.settings.smart_gaps,
    );
    let assignments = tree.assignments(
        Geometry::new(area.x, area.y, area.width.max(1), area.height.max(1)),
        input.settings.gaps.inner_horizontal,
        input.settings.gaps.inner_vertical,
    );
    let assignment_by_id: HashMap<SurfaceId, Geometry> = assignments
        .into_iter()
        .map(|assignment| (assignment.surface_id, assignment.geometry))
        .collect();

    let mut planned = Vec::new();
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
                let geometry = assignment_by_id
                    .get(&surface.surface_id)
                    .cloned()
                    .unwrap_or_else(|| Geometry::new(area.x, area.y, 1, 1));
                (
                    "dwindle".to_string(),
                    geometry,
                    SurfaceLayoutParticipation::Tiled,
                )
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
        rule: LayoutRule::Dwindle,
        mode: LayoutMode::Zones,
        revision: input.revision.saturating_add(1),
        workspace_id: input.workspace_id.clone(),
        surfaces: planned,
        surface_order,
        focus_order,
    }
}

fn insert_into(
    node: &mut DwindleNode,
    surface_id: SurfaceId,
    focused_id: &SurfaceId,
    axis: SplitAxis,
    ratio: f32,
    as_first: bool,
) -> bool {
    match node {
        DwindleNode::Leaf {
            surface_id: leaf_id,
        } if leaf_id == focused_id => {
            let old = std::mem::replace(
                node,
                DwindleNode::Leaf {
                    surface_id: surface_id.clone(),
                },
            );
            let new = DwindleNode::Leaf { surface_id };
            let (first, second) = if as_first { (new, old) } else { (old, new) };
            *node = DwindleNode::Split {
                axis,
                ratio,
                first: Box::new(first),
                second: Box::new(second),
            };
            true
        }
        DwindleNode::Leaf { .. } => false,
        DwindleNode::Split { first, second, .. } => {
            insert_into(
                first,
                surface_id.clone(),
                focused_id,
                axis.clone(),
                ratio,
                as_first,
            ) || insert_into(second, surface_id, focused_id, axis, ratio, as_first)
        }
    }
}

fn remove_from(node: DwindleNode, surface_id: &SurfaceId) -> (Option<DwindleNode>, bool) {
    match node {
        DwindleNode::Leaf {
            surface_id: leaf_id,
        } => {
            if &leaf_id == surface_id {
                (None, true)
            } else {
                (
                    Some(DwindleNode::Leaf {
                        surface_id: leaf_id,
                    }),
                    false,
                )
            }
        }
        DwindleNode::Split {
            axis,
            ratio,
            first,
            second,
        } => {
            let (first_next, first_removed) = remove_from(*first, surface_id);
            let (second_next, second_removed) = remove_from(*second, surface_id);
            match (first_next, second_next) {
                (Some(first), Some(second)) => (
                    Some(DwindleNode::Split {
                        axis,
                        ratio,
                        first: Box::new(first),
                        second: Box::new(second),
                    }),
                    first_removed || second_removed,
                ),
                (Some(only), None) | (None, Some(only)) => (Some(only), true),
                (None, None) => (None, true),
            }
        }
    }
}

fn assign_node(
    node: &DwindleNode,
    area: Geometry,
    gap_h: u32,
    gap_v: u32,
    assignments: &mut Vec<DwindleAssignment>,
) {
    match node {
        DwindleNode::Leaf { surface_id } => assignments.push(DwindleAssignment {
            surface_id: surface_id.clone(),
            geometry: area,
        }),
        DwindleNode::Split {
            axis,
            ratio,
            first,
            second,
        } => {
            let ratio = clamp_ratio(*ratio);
            match axis {
                SplitAxis::Horizontal => {
                    let gap = clamp_gap(gap_h, area.width);
                    let available = area.width.saturating_sub(gap).max(2);
                    let first_width = ((available as f32) * ratio).round() as u32;
                    let first_width = first_width.clamp(1, available.saturating_sub(1).max(1));
                    let second_width = available.saturating_sub(first_width).max(1);
                    let first_area = Geometry::new(area.x, area.y, first_width, area.height.max(1));
                    let second_area = Geometry::new(
                        area.x
                            .saturating_add(clamped_i32(first_width.saturating_add(gap))),
                        area.y,
                        second_width,
                        area.height.max(1),
                    );
                    assign_node(first, first_area, gap_h, gap_v, assignments);
                    assign_node(second, second_area, gap_h, gap_v, assignments);
                }
                SplitAxis::Vertical => {
                    let gap = clamp_gap(gap_v, area.height);
                    let available = area.height.saturating_sub(gap).max(2);
                    let first_height = ((available as f32) * ratio).round() as u32;
                    let first_height = first_height.clamp(1, available.saturating_sub(1).max(1));
                    let second_height = available.saturating_sub(first_height).max(1);
                    let first_area = Geometry::new(area.x, area.y, area.width.max(1), first_height);
                    let second_area = Geometry::new(
                        area.x,
                        area.y
                            .saturating_add(clamped_i32(first_height.saturating_add(gap))),
                        area.width.max(1),
                        second_height,
                    );
                    assign_node(first, first_area, gap_h, gap_v, assignments);
                    assign_node(second, second_area, gap_h, gap_v, assignments);
                }
            }
        }
    }
}

fn clamp_ratio(ratio: f32) -> f32 {
    if ratio.is_finite() {
        ratio.clamp(0.05, 0.95)
    } else {
        0.5
    }
}

#[cfg(test)]
mod tests {
    use super::{DwindleNode, DwindleTree, SplitAxis};
    use crate::planner::{
        LayoutGaps, LayoutPlan, LayoutRule, PlannerInput, PlannerSettings, PlannerSurface,
    };
    use crate::Geometry;
    use de_ids::SurfaceId;

    #[test]
    fn dwindle_assigns_rectangles_from_explicit_tree() {
        let tree = sample_tree();
        let assignments = tree.assignments(Geometry::new(0, 0, 1000, 800), 0, 0);

        assert_eq!(assignments[0].surface_id, SurfaceId::new("view-a"));
        assert_eq!(assignments[0].geometry, Geometry::new(0, 0, 600, 800));
        assert_eq!(assignments[1].surface_id, SurfaceId::new("view-b"));
        assert_eq!(assignments[1].geometry, Geometry::new(600, 0, 400, 400));
        assert_eq!(assignments[2].surface_id, SurfaceId::new("view-c"));
        assert_eq!(assignments[2].geometry, Geometry::new(600, 400, 400, 400));
    }

    #[test]
    fn dwindle_insert_and_remove_keep_tree_coherent() {
        let mut tree = DwindleTree::single(SurfaceId::new("view-a"));
        assert!(tree.insert_relative(
            SurfaceId::new("view-b"),
            &SurfaceId::new("view-a"),
            SplitAxis::Horizontal,
            0.5,
            false,
        ));
        assert!(tree.insert_relative(
            SurfaceId::new("view-c"),
            &SurfaceId::new("view-b"),
            SplitAxis::Vertical,
            0.5,
            false,
        ));
        assert!(tree.remove_surface(&SurfaceId::new("view-b")));

        let assignments = tree.assignments(Geometry::new(0, 0, 1000, 800), 0, 0);

        assert_eq!(assignments.len(), 2);
        assert_eq!(assignments[0].surface_id, SurfaceId::new("view-a"));
        assert_eq!(assignments[1].surface_id, SurfaceId::new("view-c"));
    }

    #[test]
    fn dwindle_clamps_ratio_and_gaps() {
        let tree = DwindleTree {
            root: Some(DwindleNode::Split {
                axis: SplitAxis::Horizontal,
                ratio: 42.0,
                first: Box::new(DwindleNode::Leaf {
                    surface_id: SurfaceId::new("view-a"),
                }),
                second: Box::new(DwindleNode::Leaf {
                    surface_id: SurfaceId::new("view-b"),
                }),
            }),
        };

        let assignments = tree.assignments(Geometry::new(0, 0, 10, 8), 100, 100);

        assert_eq!(assignments.len(), 2);
        for assignment in assignments {
            assert!(assignment.geometry.width > 0);
            assert!(assignment.geometry.height > 0);
        }
    }

    #[test]
    fn dwindle_planner_uses_explicit_tree_state() {
        let mut input = PlannerInput::new(Geometry::new(0, 0, 1000, 800), "workspace-1");
        input.rule = LayoutRule::Dwindle;
        input.settings = PlannerSettings {
            gaps: LayoutGaps::none(),
            nmaster: 1,
            mfact: 0.5,
            smart_gaps: true,
        };
        input.surfaces = vec![
            PlannerSurface::tiled(SurfaceId::new("view-a"), 0),
            PlannerSurface::tiled(SurfaceId::new("view-b"), 1),
            PlannerSurface::tiled(SurfaceId::new("view-c"), 2),
        ];
        input.focus_order = vec![
            SurfaceId::new("view-a"),
            SurfaceId::new("view-b"),
            SurfaceId::new("view-c"),
        ];
        input.dwindle_tree = Some(sample_tree());

        let plan = LayoutPlan::plan(&input).expect("dwindle should plan from explicit tree");

        assert_eq!(plan.rule, LayoutRule::Dwindle);
        assert_eq!(plan.surfaces[0].zone_id, "dwindle");
        assert_eq!(
            plan.surfaces[0].desired_geometry,
            Geometry::new(0, 0, 600, 800)
        );
        assert_eq!(
            plan.surfaces[1].desired_geometry,
            Geometry::new(600, 0, 400, 400)
        );
        assert_eq!(
            plan.surfaces[2].desired_geometry,
            Geometry::new(600, 400, 400, 400)
        );
    }

    fn sample_tree() -> DwindleTree {
        DwindleTree {
            root: Some(DwindleNode::Split {
                axis: SplitAxis::Horizontal,
                ratio: 0.6,
                first: Box::new(DwindleNode::Leaf {
                    surface_id: SurfaceId::new("view-a"),
                }),
                second: Box::new(DwindleNode::Split {
                    axis: SplitAxis::Vertical,
                    ratio: 0.5,
                    first: Box::new(DwindleNode::Leaf {
                        surface_id: SurfaceId::new("view-b"),
                    }),
                    second: Box::new(DwindleNode::Leaf {
                        surface_id: SurfaceId::new("view-c"),
                    }),
                }),
            }),
        }
    }
}

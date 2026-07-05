# Structured Window Handling

Status: design record for Den task 4237.

## Recommendation

Agora-de should make an agent-tiled workspace the first structured window
model. The default workspace should be a deterministic set of compositor-owned
zones with no arbitrary overlap among normal work surfaces. Freeform decorated
windows remain an escape hatch for dialogs, unusual tools, and human-directed
inspection, but they are not the primary agent work model.

The first product shape is a two-level model:

1. Workspaces contain named zones, initially `primary`, `secondary`, and
   `transient`.
2. Normal launched apps enter tiled zones in stable order. Dialogs and explicit
   human overrides may float in `transient`.

This gives agents a smaller state space than ad hoc overlapping windows:
targets can be addressed by zone and stable surface label, captures do not hide
state behind another window, focus changes are predictable, and test evidence
can assert geometry and visibility instead of asking a reviewer to interpret a
random stack.

Niri-style scrollable columns remain an attractive later model because they
bound horizontal chaos while preserving history, but agora-de should first own
the contract above the compositor backend. A Niri-like backend or Smithay/Rust
compositor is a backend decision, not the domain model.

## Mode Comparison

| Mode | Agent value | Limits |
| --- | --- | --- |
| Decorated freeform stack | Familiar to humans; useful for unsupported apps and transient inspection. Server-side decorations already make windows visible. | Occlusion, off-screen drift, z-order ambiguity, and unstable screenshot targets make it noisy for agents. |
| Tiled zones | Stable geometry, low occlusion risk, predictable focus/order, easier capture assertions. | Needs backend-owned layout state and truthful post-layout geometry. Stock Wayfire config is not enough as the API. |
| Scrollable columns | Good fit for long-running agent work: ordered history, bounded overlap, spatial memory. | Likely needs deeper compositor ownership or a backend with native column semantics. Prototype after the zone contract exists. |

## Live Experiment

On 2026-07-05, den-k8 was tested through the installed Wayfire service:

1. Enabled Wayfire `simple-tile` with `tile_by_default = all`.
2. Launched `Alacritty.desktop` and `foot.desktop` through
   `/api/catalog/launch`.
3. Captured `HDMI-A-1` through `agora-de-compositorctl output capture`.
4. Closed both native surfaces through `agora-de-compositorctl surface close`.
5. Restored the installed config to decoration-only behavior.

Result:

- Capture artifact:
  `/run/agent-os/artifacts/den-k8-structured-layout-4237/output-capture-1783242506875502771-16/output-capture-1783242506875502771-16.png`
- Visual inspection classified the output capture as `visible`.
- The capture showed two real native terminal surfaces placed side by side.
- Bridge geometry still reported both surfaces near the same origin:
  `Alacritty` at `x=96 y=66 width=804 height=634`; `foot` at
  `x=96 y=66 width=700 height=530`.

Conclusion: Wayfire can visibly produce structured placement for real native
surfaces, but the current bridge contract does not report post-layout geometry
truth. The successor implementation must treat truthful geometry/order/layout
events as part of the compositor backend contract, not as shell-side inference.

The repeatable config helper for this experiment lives at
`deploy/compositor/agora-de-wayfire-structured-layout-config`.

## Contract Shape

The bridge should expose a generated layout contract instead of adding one-off
shell routes. Initial state should include:

- surfaces: stable id, stable short label, app id, title, role, owner uid,
  visibility, focus, output, workspace, zone, floating/tiled state, geometry,
  and last layout revision;
- workspaces: id, name, output, active flag, zone list, ordered surface list;
- layout mode: `freeform`, `zones`, or future `columns`;
- layout revision: monotonically increasing revision for evidence correlation.

Initial actions:

- `layout.get` over bridge method `get_layout`
- `layout.set_mode` over bridge method `set_layout_mode`
- `surface.focus` over bridge method `focus_surface`
- `surface.close` over bridge method `close_surface`
- `surface.move_resize` over bridge method `move_resize_surface`
- `surface.tile` over bridge method `tile_surface`
- `surface.set_floating` over bridge method `set_surface_floating`
- `surface.assign_zone` over bridge method `assign_surface_zone`
- `surface.maximize` over bridge method `maximize_surface`
- `surface.minimize` over bridge method `minimize_surface`
- `surface.fullscreen` over bridge method `fullscreen_surface`
- `workspace.activate` over bridge method `activate_workspace`

Shellui should expose layout state and actions as a projection of this bridge
contract. TypeScript may render the controls and badges, but it should not own
placement policy.

## Agent-Visible Overlays

Structured layout should include optional overlays that are visible in capture
evidence:

- numbered surface labels;
- app id/title badges;
- active-window outline;
- zone grid and zone names;
- bounds hints for current geometry;
- focus and input-owner indication.

These overlays should be compositor or shell-overlay owned. Avoid webview
wrappers around native apps unless a concrete compositor-supported embedding
path is proven; wrapping would add another failure surface without solving
native geometry authority.

## Evidence Harness Requirements

The live harness should run against the installed den-k8 service, not VMs.
It must:

- launch at least two native apps through the shell/catalog path;
- wait for mapped, visible, capturable toplevel surfaces;
- assert focus can move to each target surface;
- capture the output and reject blank/black captures;
- assert normal tiled surfaces do not materially overlap, except explicit
  transient/floating surfaces;
- compare bridge-reported geometry with capture-visible placement when possible;
- close launched surfaces and confirm they unmap or become stale;
- export a compact JSON summary with surface ids, app ids, focus state,
  geometry, overlap result, capture path, and cleanup result.

The harness should fail separately for visibility, focus, occlusion/overlap,
geometry drift, and cleanup so planner decisions can tell backend bugs from
shell projection bugs.

## Backend Decision Criteria

Wayfire remains acceptable for the first implementation if its backend path can
provide all of the following without fragile shell inference:

- authoritative post-layout geometry events;
- commandable move/resize/tile/zone/workspace actions;
- stable workspace and focus order;
- overlay or annotation support visible in capture;
- cleanup and stale-surface behavior with classified errors;
- a bounded plugin surface that does not grow back into the predecessor god
  object.

Move layout ownership toward a Rust compositor or Smithay spike if:

- Wayfire can display a layout but cannot report truthful geometry/order;
- layout actions require synthetic keyboard shortcuts instead of protocol
  commands;
- overlays need compositor internals that are awkward in the Wayfire plugin;
- Niri-like columns become the core product model;
- the custom Wayfire plugin grows beyond the deny-plus-geometry core recorded
  in `docs/compositor-backend-decision.md`.

## Follow-Up Tasks

- Den 4247: bridge structured layout action contract.
- Den 4248: shellui structured layout state/actions.
- Den 4249: live structured-layout evidence harness.
- Den 4250: agent-visible window labels and bounds overlay.
- Den 4251: Wayfire layout plugin path versus Rust compositor ownership
  decision.

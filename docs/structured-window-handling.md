# Structured Window Handling

Status: design record for Den task 4237, with live harness follow-through for
Den task 4249 and backend ownership decision follow-through for Den task 4251.

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
compositor is a backend decision, not the domain model. After Den 4272, the
near-term domain model is a backend-neutral Rust layout planner that can compute
tiling rules before Wayfire, Smithay, or another backend applies the result.

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

The shellui projection uses `/api/layout` for camelCase layout state and
`/api/layout/action` for layout-mode and zone actions. Surface-only controls
such as focus and close stay on `/api/surfaces/action`; workspace activation
stays on `/api/workspaces/action`.

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

## Overlay Prototype

Den task 4250 adds the first shell-owned prototype:

- `agora-de-native-overlay` renders a transparent GTK4/Cairo top layer-shell
  surface identified by the `?surface=overlay` route.
- `deploy/shellui/agora-de-shell-overlay.user.service` starts it as
  `io.agorade.ShellOverlay`.
- The overlay polls `/api/layout` and `/api/surfaces`.
- Work surfaces with geometry receive visible numbered labels, app/title badges,
  active/focus outline, zone hints, and bounds text.
- `./harness/live/check-overlay-labels.py` launches two native apps, verifies the
  overlay surface is mapped, focuses each app, captures after focus changes, and
  closes launched surfaces. Capture evidence must prove both overlay annotation
  pixels and native app pixels, so mapped-only or WebKit-occluding overlays do
  not close the claim.

This is intentionally not a native-app wrapper. It is a separate shell-owned
layer, so native clients remain normal compositor surfaces.

Wayfire follow-up path:

- keep the native shell overlay as the near-term capture-visible evidence layer;
- replace CSS-derived zone hints with compositor-provided zone rectangles once
  Wayfire reports truthful post-layout geometry;
- move labels/bounds into a small Wayfire plugin only if it can draw overlays
  without taking pointer focus or depending on WebKit transparency behavior.

Rust compositor follow-up path:

- make labels, bounds, focus outline, and zone grid first-class compositor
  annotations in the layout model;
- emit the same generated layout contract to shellui for panel/status rendering;
- keep screenshot/capture evidence authoritative by drawing overlays in the
  compositor scene graph after native surfaces and before the final output.

## Evidence Harness Requirements

The live harness lives at `./harness/live/check-structured-layout.py` and runs
against the installed den-k8 service, not VMs. It must:

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

Den task 4251 records the current decision in
`docs/compositor-backend-decision.md`: Wayfire remains the installed evidence
backend, but stock Wayfire/simple-tile is not accepted as structured layout
authority. A narrow Wayfire plugin proof may keep that path only if it satisfies
the criteria above within the documented churn budget; otherwise layout
ownership moves to a Rust compositor or Smithay spike scoped to layout
authority.

Den task 4267 adds `docs/wayfire-layout-authority-probe.md` and
`compositor/protocol-fixtures/wayfire/layout-authority-probe-4267.json`. The
probe concludes that direct Wayfire APIs are sufficient to attempt the next
state bridge proof, while simple-tile remains evidence only and not the
authority API.

Den task 4268 adds the backend `layout_state` plugin event consumed by the Go
bridge. When present, `get_layout` returns this compositor-provided state
instead of the surface-derived fallback projection.

The same task also adds the minimal Wayfire placement adapter used by the live
proof. `assignZone` and `tile` send a compositor plugin `place_surface` command,
wait for `place_response`, and then expose the post-placement geometry through
`get_layout`. On den-k8, `harness/live/check-structured-layout.py` passed with
Alacritty in `primary` at `x=0,width=1280` and foot in `secondary` at
`x=1280,width=1280`, with output capture evidence under
`/run/agent-os/artifacts/den-k8-structured-layout-4268/`.

Den task 4269 adds `harness/live/check-layout-commands.py` for the command-side
proof. The harness launches native surfaces through shellui, then uses
`agora-de-compositorctl` for focus, `assign-zone`, `layout get`, unsupported
command classification, output capture, and close. On den-k8 it passed with
Alacritty and foot assigned to distinct primary/secondary zones and capture
evidence under `/run/agent-os/artifacts/den-k8-layout-commands-4269/`.

Den task 4272 closes the backend proof series. Wayfire remains the current
backend because the proof passed within the Den 4251 churn budget, while the
next layout behavior work moves into a backend-neutral Rust layout planner. The
planner should learn from `/home/research/mango`: keep selectable layout rules
separate from backend surface application, keep master-stack ratios and gap
policy as planner inputs, and represent recursive split layouts such as dwindle
as explicit state before assigning rectangles. See
`docs/backend-agnostic-layout-planner.md`.

## Follow-Up Tasks

- Den 4247: bridge structured layout action contract.
- Den 4248: shellui structured layout state/actions.
- Den 4249: live structured-layout evidence harness.
- Den 4250: agent-visible window labels and bounds overlay.
- Den 4251: Wayfire layout plugin path versus Rust compositor ownership
  decision recorded in `docs/compositor-backend-decision.md`.
- Den 4267: Wayfire layout-authority API probe recorded in
  `docs/wayfire-layout-authority-probe.md`.

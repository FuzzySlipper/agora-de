# Transient And Dialog Policy

Status: closed policy and evidence record for Den 4570.

This document defines how agora-de treats dialogs, menus, popovers, browser
prompts, file chooser windows, shell popups, unmanaged helper views, and
explicit floating overrides. The goal is to keep normal application windows
deterministic for users and agents while letting short-lived helper surfaces
stay usable without being tiled.

## Decision

Agora-de keeps transient and dialog surfaces out of the deterministic tiling
set.

Normal app toplevels are tiled when the current layout mode is tiled. Transient
surfaces are freeform and assigned to the `transient` zone. Explicit human
floating overrides are also freeform and assigned to `transient`, but they keep
the `floating` participation rather than being classified as `transient`.

The compositor backend owns truthful role, geometry, focus, workspace, and
lifecycle state. Shellui and live harnesses project those facts; they do not
infer transient policy from screenshots or browser bounds.

## Current Inventory

Bridge classification lives in `go/internal/compositorbridge/lifecycle.go`.

Current bridge rules and projected policy classes:

- `layer_shell` surfaces become `freeform` + `transient` in the `chrome` zone
  with `policyClass=shell_chrome`.
- Shell-managed surfaces whose app id starts with `io.agorade.Shell` become
  `freeform` + `transient` in the `transient` zone unless already layer-shell,
  with `policyClass=transient`.
- Explicit floating surfaces are preserved as `freeform` + `floating` in the
  `transient` zone with `policyClass=floating_override`.
- XDG roles containing `dialog`, `modal`, `popup`, `popover`, `menu`,
  `tooltip`, `transient`, or `unmanaged` become `freeform` + `transient` in the
  `transient` zone. If a dialog-like role has no backend-reported parent, it
  projects `policyClass=no_parent`.
- In `freeform` layout mode, ordinary non-shell work surfaces are classified as
  `floating` participation but keep `policyClass=work`.
- In tiled layout modes, ordinary non-shell work surfaces become `tiled` and
  participate in auto-layout with `policyClass=work`.

The generated protocol now includes `SurfacePolicyClass` and layout surfaces
carry `parentSurfaceId`, `policyClass`, and `policyReason`. The Go bridge and
shell routes expose the same facts through `agora-de-compositorctl`,
`/api/layout`, and `/api/surfaces`.

Shell projection in `go/internal/shellui/server/server.go` uses the projected
`policyClass` first, then conservative role and zone fallbacks, so the taskbar
excludes transient, shell chrome, no-parent, stale, and unsupported surfaces
from normal running-app controls.

Live evidence currently includes `harness/live/check-popup-stability.py`, which
checks shell launcher/status popup geometry, work-surface geometry stability,
closed-popup cleanup, policy projection, optional native dialog probing, and
unmanaged XDG helper classification.

Focused Go coverage currently includes:

- unmanaged XDG helpers stay transient and auto-layout excluded;
- normal browser/file-manager toplevels tile;
- shell launcher and `dialog` roles stay freeform/transient;
- shell chrome, work, parented dialog, no-parent file chooser/menu, explicit
  floating, and backend-limited unsupported responses project policy classes;
- explicit `setFloating` moves a normal surface into freeform/floating and can
  return it to tiling.

## Surface Classes

### Tiled Work Surface

Use for ordinary app toplevels that should participate in deterministic layout.

Expected state:

- `surfaceKind`: `xdg_view`
- role is empty or `toplevel`
- `layoutRole`: `tiled`
- `layoutMode`: `zones` or another tiled mode
- `zoneId`: a work zone such as `master`, `stack`, `primary`, or `secondary`

Examples: terminal windows, browsers, file managers, editors, and normal app
main windows.

### Transient Surface

Use for short-lived or helper surfaces that should not disturb tiled work
geometry.

Expected state:

- `layoutRole`: `transient`
- `layoutMode`: `freeform`
- `zoneId`: `transient`, except layer-shell chrome which uses `chrome`
- `policyClass`: `transient`, `shell_chrome`, or `no_parent`
- may receive overlay labels and capture evidence, but does not appear in the
  taskbar work-surface list

Examples: dialogs, modals, popovers, menus, tooltips, browser prompts, file
chooser dialogs, unmanaged helper views, shell launcher/status popups, shell
overlay, shell background, and shell panel.

### Explicit Floating Surface

Use when a normal work surface was intentionally floated by a human or agent.

Expected state:

- `layoutRole`: `floating`
- `layoutMode`: `freeform`
- `zoneId`: `transient`
- remains visible and addressable by surface id
- can return to tiling through `setFloating(false)`

This is an override, not a classifier guess. It must not be silently converted
to `transient`, because agents and users need to know the surface can be tiled
again.

## Parent And Focus Policy

Parent-child dialog relationships are desired but not yet authoritative.

Policy target:

- If the backend reports a parent surface id, a dialog follows the parent's
  output and workspace.
- If no parent is known, the dialog remains transient on its reported output
  and workspace, and the state should expose a `no-parent` classification
  rather than pretending it is a normal work surface.
- Transient focus should not reorder tiled surfaces or promote a dialog into
  the master area.
- Closing a parent should not leave stale child dialogs as active work surfaces.

Current limitation: the bridge does not yet expose parent ids or transient
parent ids when the current Wayfire bridge readback omits them. The policy
class still distinguishes `no_parent` from work surfaces. Follow-up backend
work should add only backend-reported parent ids or explicit classified
outcomes; it should not infer parenthood from geometry or titles.

## Failure Classes

Transient/dialog failure cases should be classified so agents can tell policy
from backend defects:

- `stale`: the process or surface disappeared, but a cached record was still
  visible briefly.
- `unsupported`: the backend cannot perform a requested transient/dialog action
  for this surface kind.
- `no-parent`: a dialog-like surface has no backend-reported parent surface.
- `backend-limited`: the backend exposes the surface but lacks needed role,
  parent, stacking, or state-change authority.

These classes should appear in action responses, diagnostics, live evidence, or
documented harness skips. They should not be represented by jittery layout,
silent tiling, or screenshot-only assertions.

Implemented projection:

- `stale` remains the action/error class for vanished or invisible surfaces.
- `unsupported` remains the action/error class for unsupported backend
  requests.
- `no_parent` is a surface policy class for dialog-like roles without a parent.
- `backend_limited` is projected on accepted unsupported layout responses when
  the target surface is valid but backend geometry authority is unavailable.

## Agent Rules

Agents should use structured state:

- Read `role`, `layoutRole`, `layoutMode`, `zoneId`, `workspaceId`,
  `outputId`, `focused`, `visible`, `geometry`, `parentSurfaceId`,
  `policyClass`, and `policyReason` from `/api/layout`, `/api/surfaces`, or
  `agora-de-compositorctl`.
- Treat `layoutRole=transient` as helper chrome/dialog state, not a tiled work
  target.
- Treat `layoutRole=floating` in `zoneId=transient` as an explicit override that
  can be returned to tiling.
- Use screenshots and output captures only as evidence after structured ids are
  known.
- Do not infer dialog status from window position, overlap, browser bounds, or
  title text alone.

## Evidence Path

Installed-service evidence for this track:

- shell launcher/status popups remain non-exclusive, stable, and cleaned up;
- unmanaged helper views stay transient and excluded from auto-layout;
- normal app toplevels continue to tile deterministically while transient
  surfaces are open;
- at least one representative native dialog, browser prompt, menu, or file
  chooser case is classified as transient when a stable reproducer exists on
  den-k8;
- capture evidence may prove visibility, but structured routes and compositor
  state remain authority.

If a representative native dialog cannot be produced reliably on den-k8, the
live harness should record a skip with the missing app capability or backend
limitation rather than fabricating a screenshot-only proof.

Closeout evidence recorded for Den 4660:

```bash
./harness/live/check-popup-stability.py \
  --base-url http://127.0.0.1:17780 \
  --cycles 1 \
  --baseline-samples 1 \
  --open-samples 2 \
  --closed-samples 1 \
  --sample-delay-seconds 0.25 \
  --launch-delay-seconds 0.75 \
  --cleanup-delay-seconds 0.5 \
  --samples-output /tmp/agora-de-4660-popup-policy-samples.jsonl
```

Result on the installed den-k8 service: `10 passed / 0 failed / 1 skipped`.
The harness observed `io.agorade.ShellStatus` and
`io.agorade.ShellLauncher` as `policyClass=shell_chrome`, stable,
non-exclusive layer-shell popups and verified cleanup. The skipped check was
`native-dialog-capability`, because no representative native dialog app id was
provided for that run.

Final validation:

- `python3 -m py_compile harness/live/check-popup-stability.py`
- `./harness/ci/check-live-harnesses.sh`
- `./harness/ci/check-all.sh`
- GitHub `Verify Agora DE` for the closeout commits

## Non-Goals

- Do not add a shell runtime mode for dialogs.
- Do not tile transient surfaces to make screenshots easier.
- Do not make shellui own parent/following or stacking policy.
- Do not import predecessor agora-os internals.
- Do not add VM orchestration for this track.

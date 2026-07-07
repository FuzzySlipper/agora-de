# Transient And Dialog Policy

Status: initial policy and inventory for Den 4570.

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

Current bridge rules:

- `layer_shell` surfaces become `freeform` + `transient` in the `chrome` zone.
- Shell-managed surfaces whose app id starts with `io.agorade.Shell` become
  `freeform` + `transient` in the `transient` zone unless already layer-shell.
- Explicit floating surfaces are preserved as `freeform` + `floating` in the
  `transient` zone.
- XDG roles containing `dialog`, `modal`, `popup`, `popover`, `menu`,
  `tooltip`, `transient`, or `unmanaged` become `freeform` + `transient` in the
  `transient` zone.
- In `freeform` layout mode, ordinary non-shell work surfaces are classified as
  `floating`.
- In tiled layout modes, ordinary non-shell work surfaces become `tiled` and
  participate in auto-layout.

Shell projection mirrors the same conservative role markers in
`go/internal/shellui/server/server.go` so the taskbar excludes transient and
chrome surfaces from normal running-app controls.

Live evidence currently includes `harness/live/check-popup-stability.py`, which
checks shell launcher/status popup geometry, work-surface geometry stability,
closed-popup cleanup, and unmanaged XDG helper classification.

Focused Go coverage currently includes:

- unmanaged XDG helpers stay transient and auto-layout excluded;
- normal browser/file-manager toplevels tile;
- shell launcher and `dialog` roles stay freeform/transient;
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
failure classes. Follow-up implementation should add only backend-reported
facts or explicit classified outcomes; it should not infer parenthood from
geometry or titles.

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

## Agent Rules

Agents should use structured state:

- Read `role`, `layoutRole`, `layoutMode`, `zoneId`, `workspaceId`,
  `outputId`, `focused`, `visible`, and `geometry` from `/api/layout`,
  `/api/surfaces`, or `agora-de-compositorctl`.
- Treat `layoutRole=transient` as helper chrome/dialog state, not a tiled work
  target.
- Treat `layoutRole=floating` in `zoneId=transient` as an explicit override that
  can be returned to tiling.
- Use screenshots and output captures only as evidence after structured ids are
  known.
- Do not infer dialog status from window position, overlap, browser bounds, or
  title text alone.

## Evidence Path

Minimum installed-service evidence for this track:

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

## Non-Goals

- Do not add a shell runtime mode for dialogs.
- Do not tile transient surfaces to make screenshots easier.
- Do not make shellui own parent/following or stacking policy.
- Do not import predecessor agora-os internals.
- Do not add VM orchestration for this track.

# Multi-Output Workspace Policy

Status: initial policy for Den 4569.

This document defines how agora-de should model workspaces when more than one
physical output is present. It keeps the current den-k8 single-output behavior
stable while giving agents and later shell controls a clear target model.

## Decision

Agora-de uses output-scoped workspace sets.

Each physical output has its own active workspace. A workspace belongs to one
output, has one stable `workspaceId`, and reports its owning `outputId`.
Surfaces belong to an `outputId` and a `workspaceId`; agents target surfaces and
workspace actions through those structured ids rather than screenshot position.

The workspace set is not a single global desktop stretched across monitors. It
is also not a shell-owned virtual desktop abstraction. The compositor backend
owns output/workspace membership, and agora-de projects that backend state.

## Compatibility

The current single-output shape remains valid:

- `workspace-1` remains the default workspace id on the installed one-output
  den-k8 session.
- `/api/workspaces` continues to expose `currentWorkspaceId` and `workspaces`.
- `/api/layout` continues to expose `surfaces[]` and `workspaces[]` with
  `outputId`, `workspaceId`, zone membership, surface order, focus, visibility,
  and geometry.
- Existing `POST /api/workspaces/action {"workspaceId":"workspace-1",
  "action":"activate"}` remains valid.

Multi-output support must extend this shape without breaking those fields.

## Identity Rules

Output ids come from the compositor backend and are treated as stable within a
running session. Examples include `HDMI-A-1` and `DP-1`.

Workspace ids must be globally unique in the agora-de API even though the human
display name may repeat per output. On the single-output installed session, the
canonical id is still `workspace-1`. On multi-output sessions, the backend
adapter may keep compositor-native ids if they are globally unique; otherwise it
must namespace them by output before exposing them.

Workspace names are display labels, not authority. It is fine for two outputs to
both display `workspace 1` as long as their `workspaceId` values are distinct or
the backend proves they are globally unique.

## Agent Target Model

Agents should treat this tuple as authoritative:

```text
outputId + workspaceId + surfaceId
```

Rules:

- Use `outputId` from `/api/layout`, `/api/surfaces`, or
  `agora-de-compositorctl output list` to choose the monitor.
- Use `workspaceId` from `/api/layout.workspaces[]` or `/api/workspaces` to
  choose the workspace.
- Use `surfaceId` from `/api/surfaces` or `/api/layout.surfaces[]` to choose a
  window.
- Use screenshots and output captures only as evidence after structured ids are
  known.
- Never infer output/workspace membership from browser bounds or shell webview
  rectangles.

## Actions

Existing actions remain:

- `POST /api/workspaces/action` activates a workspace by stable `workspaceId`.
- `POST /api/layout/action` with `assignZone`, `tile`, `moveResize`,
  `setFloating`, and `activateWorkspace` remains backend-owned.
- `agora-de-compositorctl workspace activate --workspace <workspaceId>` remains
  the CLI activation path.

If a future backend exposes ambiguous local workspace ids, shellui and
compositorctl should add an optional `outputId`/`--output` selector at the
boundary. Do not make shellui disambiguate by screen geometry.

Moving a surface between outputs is a separate backend action from activating a
workspace. The target state for that action is explicit:

```text
surfaceId -> outputId + workspaceId + zoneId
```

If the current backend cannot move a surface across outputs, it must return a
classified `backend_unsupported` result rather than simulating the move in
shell state.

## Shell Chrome And Work Areas

Shell chrome is output-aware but not a work surface:

- panel, launcher, status, overlay, background, dialogs, menus, and unmanaged
  helper views remain transient/floating;
- layer-shell surfaces report `outputId` and geometry as evidence;
- each output work area is computed from backend output bounds minus the shell
  reservations that actually apply to that output;
- absent shell chrome on an output must not invent a reservation there.

The shell may choose a compact single-output taskbar presentation when only one
output exists. Multi-output UI should show enough output/workspace identity for
humans to avoid accidental activation, but shell controls must call backend
actions and render backend state rather than owning placement policy.

## Contract Inventory

Existing fields already needed for the policy:

- `LayoutSurface.outputId`
- `LayoutSurface.workspaceId`
- `LayoutWorkspace.outputId`
- `LayoutWorkspace.active`
- `LayoutWorkspace.surfaceOrder`
- `SurfaceView.outputId`
- `SurfaceView.workspaceId`
- `LogicalOutput.name`
- output capture by output name

Likely extensions:

- `/api/workspaces` should expose output identity for each workspace and, when
  useful, the currently focused/current output id.
- workspace actions may need an optional output selector only if backend
  workspace ids are not globally unique.
- live harnesses should classify true multi-output assertions as skipped when
  the installed host exposes one output, while still proving output discovery
  and single-output stability.

Avoid adding hand-written protocol mirrors. If generated protocol output needs
new fields, extend the Rust contract/codegen path and regenerate.

## Evidence Path

Minimum installed-service evidence:

- `agora-de-compositorctl output list` reports output ids and surface
  membership.
- `/api/layout` reports workspace and surface `outputId`.
- `/api/workspaces` reports workspace identity, active state, and surface
  counts.
- workspace activation works on the current single-output install.
- output capture works by explicit output name.

Additional evidence when multiple outputs are available:

- each output reports its own active workspace;
- activating a workspace changes only that output's workspace set unless the
  backend explicitly reports a global switch;
- a moved surface reports the new `outputId`, `workspaceId`, zone, and
  post-placement geometry;
- capture can target each physical output by name.

## Non-Goals

- Do not add VM orchestration to agora-de for this track.
- Do not require agora-os governance services.
- Do not add a shell runtime mode for multi-output behavior.
- Do not infer output or workspace authority from screenshots.
- Do not preserve predecessor agora-os runtime compatibility as a constraint.

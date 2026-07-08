# Post-Northstar Smithay Evaluation

Status: Smithay/Rust backend work remains deferred as the installed backend.
Wayfire remains the current installed evidence backend while it keeps satisfying
the deployed WM contract behind the backend-neutral boundary.

This is a post-northstar decision record. It revisits the earlier Smithay spike
after Agora DE proved a usable tiled desktop with native launch, shell chrome,
workspace controls, overlay evidence, live recovery, and soak harnesses.

## Decision

Keep Wayfire as the installed backend and pursue Smithay only as a nested
runtime proof when a reopen trigger fires or a dedicated backend research window
is explicitly chosen.

Smithay is still architecturally attractive because it can move compositor
authority into Rust. It is not yet the lower-risk path for the installed
desktop because Agora DE now depends on a broad deployed contract, not only
primitive layout placement:

- native client lifecycle and trusted surface ids;
- governed desktop-entry launch completion;
- layout mode/rule/settings/revision and command result classes;
- workspace activation, output-scoped state, surface order, and focus order;
- shell chrome, shell popups, dialogs, transients, and explicit tiling
  exclusions;
- output capture and visible pixel classification;
- agent-facing state/actions and overlay labels;
- install, restart, recovery, cleanup, and soak evidence.

The machine-readable record is
`compositor/protocol-fixtures/smithay/post-northstar-evaluation-4572.json`.
The expanded backend matrix is
`compositor/protocol-fixtures/capabilities/backend-capability-matrix.json`.

## Capability Result

| Area | Wayfire Installed Backend | Smithay/Rust Backend |
| --- | --- | --- |
| Native clients | Proven by installed launch/layout/live harnesses | Missing runtime proof |
| Layout authority | Proven through Rust planner plus Wayfire acknowledgement | Prototype shape only |
| Workspace/focus/order | Reported through bridge and shell routes | Prototype shape only |
| Shell chrome/transients | Proven through layer-shell and policy metadata | Prototype shape only |
| Native launch visibility | Proven through shell catalog and compositor surface ids | Missing launch mapping |
| Capture evidence | Proven with `capture_visible` artifacts | Missing capture path |
| Agent controls | Proven through compositorctl and HTTP routes | Contract reusable but unimplemented |
| Deployment/recovery | Proven on den-k8 installed service | Missing nested/install path |

The standard Wayland protocol probe can shrink custom code for identity,
toplevel listing, and capture transport, but it does not provide Agora DE layout
authority, workspace semantics, transient policy, launch mapping, command
result classes, or deployment/recovery behavior.

## Responsibility Map

Current Wayfire responsibilities are intentionally split:

| Responsibility | Classification | Future Owner |
| --- | --- | --- |
| Plugin socket and control socket | Bridge transport | Go bridge now; generated/Rust backend transport later |
| Lifecycle, focus, visibility, geometry, output facts | Backend facts | Any compositor backend adapter |
| Layout mode, zones, revisions, order, result classes | Product contract | Rust layout/backend contracts |
| Master-stack, gaps, ratios, future layout algorithms | Product behavior | Rust layout planner crates |
| Shell chrome, launcher, taskbar, theme, overlay presentation | Shell projection | Shell/chrome/theme layers |
| Input denial and transient tiling exclusion | Backend policy enforcement | Policy projection plus backend enforcement |
| Capture transport and pixel classification | Evidence transport | Backend capture adapter plus live harness classifier |

The backend must not absorb shell UI, launcher, taskbar, theme, governance, or
layout heuristics. Those remain outside compositor backend product scope.

## Reopen Triggers

Reopen Smithay/Rust backend implementation if one of these becomes true:

- Wayfire layout actions require synthetic keyboard shortcuts.
- Wayfire geometry or focus must be inferred from shell state, browser
  rectangles, screenshots, or capture pixels.
- The Wayfire plugin grows into shell UI, launcher, theme, governance, or
  product layout heuristics.
- A future layout rule cannot be represented as Rust planner output plus
  backend acknowledgement.
- A nested Smithay proof hosts two native clients, maps governed launch to
  surface ids, applies the shared layout command fixture, reports
  workspace/focus/order state, and produces capture-visible evidence with lower
  backend complexity than Wayfire.

## Future Task Shape

If reopened, the next work should be deliberately nested and non-disruptive:

1. Prototype a nested Smithay native-client host behind
   `compositor-backend-api`.
2. Implement a Smithay layout command fixture adapter using the existing
   `layout-model` command semantics.
3. Add a Smithay capture-visible evidence path.

None of these should replace the installed Wayfire service until the nested path
proves native clients, launch mapping, layout commands, workspace/focus/order,
capture, cleanup, and agent affordances against the same contract.

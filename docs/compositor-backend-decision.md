# Compositor Backend Decision

Wayfire remains the initial agora-de compositor backend, but stock Wayfire
layout behavior is not accepted as agora-de layout authority.

This is a source decision, not a compatibility fallback. The backend contract
stays in Rust and the Wayfire plugin path is one implementation behind that
contract.

## Evidence Inputs

- Capability matrix:
  `compositor/protocol-fixtures/capabilities/backend-capability-matrix.json`
- Standard protocol probe:
  `compositor/standard-protocol-probe/probe-observations.json`
- Wayfire layout-authority probe:
  `compositor/protocol-fixtures/wayfire/layout-authority-probe-4267.json`
- Fixture gate:

```bash
./harness/ci/check-compositor.sh
```

## Capability Decisions

| Capability | Decision |
| --- | --- |
| Kernel credential attribution | Standard protocols can shrink the custom surface for launched or sandboxed clients through `security-context-v1`; the backend still owns trusted fallback where no scoped socket exists. |
| Surface lifecycle events | `ext-foreign-toplevel-list-v1` can reduce custom discovery where compositor support and metadata are sufficient; Wayfire plugin lifecycle remains the initial source. |
| Synchronous input deny | Keep in the custom backend core. The standard probe found no protocol that can deny input synchronously in the compositor input path. |
| Per-toplevel capture | `ext-image-copy-capture-v1` is a viable direction where source objects are exposed; Wayfire/GLES readback remains the initial backend evidence path. |
| Geometry control | Keep in the custom backend core. The standard probe found no protocol granting DE-owned per-surface move/resize authority. |
| Structured layout authority | Keep in the Rust-owned backend contract. Wayfire may remain the first implementation only if a narrow plugin proof emits truthful post-layout geometry, focus/order, workspace/zone state, and command results without shell inference or synthetic shortcuts. |

Post-northstar capability updates are tracked in the matrix after Den task 4572.
The deployed WM contract now also includes workspace state, shell chrome and
transient policy, native launch visibility, live capture evidence, agent
control affordances, and deployment/recovery operations. The matrix includes a
Smithay/Rust backend spike entry and keeps it deferred until a nested native
client proof exists.

## Backend Shape

Initial product work stays on the Wayfire plugin backend because it covers every
required capability today. Standard protocol work should be used to shrink the
custom plugin surface where the matrix already records support, especially for
scoped identity, toplevel listing, and capture transport.

Smithay or another custom compositor remains a spike until it proves a smaller
and more governable enforcement core than the Wayfire path. The irreducible core
today is synchronous input denial plus DE-owned geometry and layout authority.

After the auto-tiling WM northstar, the comparison bar is broader than geometry
alone. See `docs/post-northstar-smithay-evaluation.md` and
`compositor/protocol-fixtures/smithay/post-northstar-evaluation-4572.json`.
Wayfire remains the installed evidence backend because it currently proves the
full deployed contract: native launch mapping, layout command acknowledgement,
workspace state, shell chrome/transient classification, live capture,
agent-facing controls, recovery, and soak evidence.

## Structured Layout Decision

Den task 4251 applies the structured-window evidence from the installed den-k8
service:

- Wayfire `simple-tile` can visibly place two real native terminals side by
  side in output capture evidence.
- The bridge still reported both tiled surfaces near the same origin during
  that experiment, so stock Wayfire layout display did not prove truthful
  post-layout geometry.
- `/api/layout` can provide a shell-facing projection and fallback when the
  backend reports `backend_unsupported`, but that fallback is not compositor
  layout authority.
- The shell-owned overlay proves capture-visible labels, bounds, focus outline,
  and zone hints, but it is an evidence layer until compositor-owned geometry
  and annotation state are available.

Decision after Den 4268, Den 4269, Den 4270, and Den 4271: keep Wayfire as the
near-term installed compositor and evidence backend, and move layout policy into
a backend-neutral Rust layout planner. The Wayfire proof satisfied the current
criteria inside the Den 4251 churn budget, so the Smithay spike remains parked.
Wayfire is an adapter that applies rectangles and reports acknowledgement
geometry; it is not the owner of product layout rules.

The Wayfire proof must provide:

- authoritative post-layout geometry events for every normal work surface;
- commandable move/resize, tile, floating, zone assignment, workspace
  activation, focus, and close results through the generated backend contract;
- stable workspace surface order and focus order;
- cleanup and stale-surface classification after close/unmap;
- optional overlay or annotation support only if it is visible in capture
  without stealing pointer focus or depending on WebKit transparency behavior.

Den task 4267 checked Wayfire 0.10.1 headers and simple-tile metadata. The
probe found direct Wayfire APIs for lifecycle, post-commit geometry, focus,
workspace state, stacking order, and primitive move/resize/tile/fullscreen
commands. It did not find a public simple-tile state or command API suitable
for agora-de zones. The Wayfire proof may proceed only as an agora-de custom
plugin using direct Wayfire APIs and small backend-owned zone/revision state.

Den task 4268 added a Wayfire layout-state bridge and placement adapter. Den
task 4269 proved command execution through `agora-de-compositorctl` and
`/api/layout/action` against the installed den-k8 session. The proof did not
depend on synthetic keyboard shortcuts, screenshot-derived geometry, or shell
inference. Den task 4270 hardened Rust layout command semantics, and Den task
4271 parked the Smithay comparison spike behind explicit trigger criteria.

The proof fails in the future, and the Rust/Smithay compositor path becomes
primary, if any required layout action starts depending on synthetic keyboard
shortcuts, if the bridge must infer post-layout geometry from shell state or
screenshots, if Wayfire internals force the custom plugin beyond the scope
below, or if future layout rules cannot be expressed as backend-neutral Rust
planner output plus backend acknowledgement.

See `docs/backend-agnostic-layout-planner.md` for the next implementation
series. The important successor boundary is now: Rust owns layout rule behavior,
Wayfire applies and reports native surface geometry, and shellui projects state.

## Wayfire Plugin Scope

Allowed custom Wayfire plugin responsibilities:

- synchronous input denial and audit-facing input decisions;
- surface lifecycle, identity fallback, focus, visibility, and stale cleanup
  events not covered by standard protocols;
- authoritative geometry, workspace, zone, and layout revision events;
- backend execution of generated layout commands;
- output or per-toplevel capture while standard capture support is incomplete;
- compositor-drawn annotations only if they remain a small extension of the
  layout event model.

Explicitly out of scope:

- shell UI, launcher, dock, taskbar, and theme policy;
- layout heuristics that belong in Rust-owned layout domain crates;
- webview wrappers around native clients;
- governance log reads or OS governance policy;
- Niri-like column history as an accidental Wayfire plugin product.

Churn budget result: the Den 4267-4271 proof stayed within budget. It touched a
bounded Wayfire plugin and bridge adapter path, passed live installed-service
checks, and kept shell UI, launcher, governance, and theme policy out of the
backend. Future work is no longer proof churn; it is implementation of a
backend-neutral Rust layout planner and a Wayfire adapter that applies its
rectangles.

## Closeout Rule

A future change may move a missing capability out of the custom backend only
when the matrix entry, probe observations, and this decision record are updated
together. `check-compositor.sh` must continue to reject unsupported claims.

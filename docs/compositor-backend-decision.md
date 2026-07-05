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

## Backend Shape

Initial product work stays on the Wayfire plugin backend because it covers every
required capability today. Standard protocol work should be used to shrink the
custom plugin surface where the matrix already records support, especially for
scoped identity, toplevel listing, and capture transport.

Smithay or another custom compositor remains a spike until it proves a smaller
and more governable enforcement core than the Wayfire path. The irreducible core
today is synchronous input denial plus DE-owned geometry and layout authority.

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

Decision: keep Wayfire as the near-term installed compositor and evidence
backend, but treat structured layout authority as unproven in the Wayfire path.
The next Wayfire work must be a bounded layout-authority plugin proof, not more
shell inference. If that proof cannot satisfy the criteria below, layout
ownership moves to a Rust compositor or Smithay spike scoped to layout authority
rather than to a full desktop rewrite.

The Wayfire proof must provide:

- authoritative post-layout geometry events for every normal work surface;
- commandable move/resize, tile, floating, zone assignment, workspace
  activation, focus, and close results through the generated backend contract;
- stable workspace surface order and focus order;
- cleanup and stale-surface classification after close/unmap;
- optional overlay or annotation support only if it is visible in capture
  without stealing pointer focus or depending on WebKit transparency behavior.

The proof fails, and the Rust/Smithay path becomes primary, if any required
layout action depends on synthetic keyboard shortcuts, if the bridge must infer
post-layout geometry from shell state or screenshots, or if Wayfire internals
force the custom plugin beyond the scope below.

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

Churn budget: one narrow Wayfire layout-authority spike may touch the Wayfire
plugin and bridge adapter layers. Stop and promote the Rust/Smithay layout
spike if the proof requires more than three follow-up implementation tasks, more
than five working days, synthetic input actions, screenshot-derived geometry, or
a broad plugin object that combines shell policy with compositor authority.

## Closeout Rule

A future change may move a missing capability out of the custom backend only
when the matrix entry, probe observations, and this decision record are updated
together. `check-compositor.sh` must continue to reject unsupported claims.

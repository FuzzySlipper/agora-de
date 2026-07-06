# Backend-Agnostic Layout Planner

Status: implementation plan created during Den 4272 closeout.

## Decision

Agora-de should keep Wayfire as the current installed compositor backend, but
move layout policy into a backend-neutral Rust layout planner. Wayfire should
apply planned rectangles to real native surfaces and report post-placement
geometry, focus, lifecycle, and capture evidence. Shellui should render controls
and labels, not decide placement.

This keeps the successor boundary clean if Wayfire is later replaced by
Smithay, Niri-like columns, or another compositor path: the backend adapter
changes, but planner semantics and fixtures remain stable.

## Mango Lessons

`/home/research/mango` is useful as a reference because it separates the
interesting parts of window layout more cleanly than many compositor codebases:

- `src/layout/layout.h` registers layout algorithms by name and function.
- `src/layout/horizontal.h` and `src/layout/vertical.h` compute master-stack
  rectangles from monitor work area, visible tiled clients, gaps, master count,
  and ratios.
- `src/layout/dwindle.h` keeps recursive split state in a tree, then assigns
  rectangles from that tree.
- `docs/window-management/rules.md` keeps window/tag rule policy separate from
  the backend mechanics that resize surfaces.

Agora-de should not port Mango directly. The useful pattern is to treat layout
as pure planner state plus backend application:

1. Collect backend-neutral inputs: output work area, reserved shell chrome,
   workspace, visible normal surfaces, focus order, surface order, floating or
   tiled participation, gaps, and selected layout rule.
2. Run a Rust planner that produces desired rectangles, zone membership,
   order, focus, revision, and command results.
3. Send the desired rectangles to the backend adapter.
4. Accept backend acknowledgement geometry as final truth and emit it through
   the generated contract.

## Initial Rule Set

Start small and deterministic:

- `zones`: primary/secondary/transient assignment, already proven live.
- `master-stack`: one primary master area plus stack area, with `nmaster`,
  `mfact`, inner gaps, outer gaps, smartgaps, and stable surface order.
- `dwindle`: explicit binary split tree for recursive placement, initially
  fixture-backed before live UI controls.

Scrollable columns are intentionally later. They are likely a strong agent
workspace model, but the first goal is a backend-neutral planner boundary that
can support either Wayfire placement or a future Rust compositor.

## Den Task Series

Parent task: Den 4318, `Backend-agnostic tiling layout planner implementation
series`.

- Den 4319: define backend-neutral planner inputs and plan output.
- Den 4320: implement zone and master-stack planner rules.
- Den 4321: prototype backend-neutral dwindle split tree planner.
- Den 4322: wire the Wayfire adapter to apply Rust planner rectangles.
- Den 4323: expand live den-k8 layout harness coverage for planner-backed
  tiling.
- Den 4324: expose shell layout rule controls as planner actions.

## Adapter Boundary

Rust planner owns:

- layout rule selection and state transitions;
- expected rectangles before backend application;
- surface order, focus order, workspace membership, and revisions;
- stale/missing/backend-unsupported/invalid command result classes;
- fixtures that backend adapters must satisfy.

Wayfire adapter owns:

- mapping planned rectangles to `place_surface` requests;
- reporting post-placement acknowledgement geometry;
- compositor lifecycle and focus events;
- installed den-k8 live evidence.

Shellui owns:

- display of current layout state;
- buttons and controls that call generated layout actions;
- labels, badges, and operator feedback.

Shellui does not own tiling geometry.

## Validation Path

Local validation:

```bash
cargo test --manifest-path de-rs/Cargo.toml -p layout-model
./harness/ci/check-contracts.sh
./harness/ci/check-compositor.sh
```

Live validation stays on den-k8 against the installed service:

```bash
./harness/live/check-structured-layout.py
./harness/live/check-layout-commands.py
```

Future live checks should add three-or-more-surface cases, master-stack
geometry, dwindle insertion/removal, resize/rebalance commands, and cleanup
after surface close.

# Compositor Backend Plan

Wayfire remains the initial backend, but the backend contract is explicit and
fixture-backed. The current comparison lives in:

```text
compositor/protocol-fixtures/capabilities/backend-capability-matrix.json
```

The checked standard-protocol probe observations live in:

```text
compositor/standard-protocol-probe/probe-observations.json
```

The fixture is checked by:

```bash
./harness/ci/check-compositor.sh
```

## Required Capabilities

- kernel credential attribution;
- surface lifecycle events;
- synchronous input deny;
- per-toplevel capture;
- geometry control.

## Current Reading

The Wayfire plugin path can cover every required capability through custom
plugin support. Standard Wayland protocols are expected to cover meaningful
parts of the stack, especially credential attribution, surface lifecycle, and
per-toplevel capture. Synchronous input deny and geometry control stay marked
missing for the standard-protocol probe until a spike proves otherwise.

The structured layout decision in `docs/compositor-backend-decision.md` narrows
that reading: Wayfire remains the near-term installed backend, but stock
Wayfire layout behavior is not layout authority. The Den 4268/4269 proof showed
that a bounded Wayfire adapter can emit truthful post-layout geometry,
workspace/zone state, focus/order, and command results without shell inference.
The next implementation step is a backend-neutral Rust layout planner whose
rectangles can be applied by Wayfire now and by another backend later.

The standard-protocol observation artifact currently maps:

- `security-context-v1` / `wp_security_context_v1` to scoped app/sandbox
  identity for launched or sandboxed clients;
- `ext-foreign-toplevel-list-v1` to foreign toplevel handles that can reduce
  custom surface discovery;
- `ext-image-copy-capture-v1` to capture of compositor image sources,
  including toplevels where source objects are exposed.

It does not prove a standard path for synchronous input denial or DE-controlled
surface geometry. Those remain custom-backend responsibilities.

This keeps the strategic posture concrete:

- shrink the custom compositor-specific surface when protocols can carry work;
- keep synchronous deny visible as the likely irreducible enforcement core;
- do not start a Smithay product build until the probe explains how small the
  in-compositor core really is;
- keep Smithay parked while the Wayfire adapter remains small and truthful;
- promote the Rust/Smithay layout spike only if future layout behavior cannot be
  represented as backend-neutral Rust planner output plus backend acknowledgement.

The planner series is recorded in
`docs/backend-agnostic-layout-planner.md`. The useful lesson from Mango is
architectural rather than literal: layout algorithms should be selectable
planner functions over output/workspace/surface state, with backend code limited
to applying and acknowledging rectangles.

## Updating The Matrix

Update the JSON fixture, standard-protocol probe observations, and Rust reports
together. A change that marks a previously missing capability as supported must
include spike evidence in the fixture's `evidence` field and a short doc note
explaining what changed.

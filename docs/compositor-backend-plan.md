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
  in-compositor core really is.

## Updating The Matrix

Update the JSON fixture, standard-protocol probe observations, and Rust reports
together. A change that marks a previously missing capability as supported must
include spike evidence in the fixture's `evidence` field and a short doc note
explaining what changed.

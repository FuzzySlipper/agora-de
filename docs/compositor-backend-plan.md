# Compositor Backend Plan

Wayfire remains the initial backend, but the backend contract is explicit and
fixture-backed. The current comparison lives in:

```text
compositor/protocol-fixtures/capabilities/backend-capability-matrix.json
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

This keeps the strategic posture concrete:

- shrink the custom compositor-specific surface when protocols can carry work;
- keep synchronous deny visible as the likely irreducible enforcement core;
- do not start a Smithay product build until the probe explains how small the
  in-compositor core really is.

## Updating The Matrix

Update the JSON fixture and the Rust reports together. A change that marks a
previously missing capability as supported must include spike evidence in the
fixture's `evidence` field and a short doc note explaining what changed.

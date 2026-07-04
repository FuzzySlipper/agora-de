# Compositor Backend Decision

Wayfire remains the initial agora-de compositor backend.

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

## Backend Shape

Initial product work stays on the Wayfire plugin backend because it covers every
required capability today. Standard protocol work should be used to shrink the
custom plugin surface where the matrix already records support, especially for
scoped identity, toplevel listing, and capture transport.

Smithay or another custom compositor remains a spike until it proves a smaller
and more governable enforcement core than the Wayfire path. The irreducible core
today is synchronous input denial plus DE-owned geometry authority.

## Closeout Rule

A future change may move a missing capability out of the custom backend only
when the matrix entry, probe observations, and this decision record are updated
together. `check-compositor.sh` must continue to reject unsupported claims.

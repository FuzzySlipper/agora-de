# Go Lift Cells

The predecessor bridge was a large mixed-responsibility daemon. The successor
lands behavior into small Go cells with machine-checked imports.

## `surfacetrack`

Path:

```text
go/internal/surfacetrack
```

Owns:

- authoritative surface lifecycle projection;
- mapped/unmapped/focused state;
- kernel-attributed owner process fields as reported by the compositor backend;
- input-denied event evidence counts.

Does not own:

- policy decisions;
- capture;
- launch/session association;
- shell HTTP routes;
- app catalog behavior.

The package consumes `go/internal/wayfireproto` and fixture-tests against:

```text
compositor/protocol-fixtures/wayfire/plugin-events.jsonl
```

## `policy`

Path:

```text
go/internal/policy
```

Owns:

- compositor policy-cache projection;
- policy replace/upsert/remove commands;
- current input actor context.

Does not own:

- surface lifecycle state;
- forced close commands;
- launch/session association;
- audit or escalation decisions.

The package consumes `go/internal/wayfireproto` and fixture-tests against:

```text
compositor/protocol-fixtures/wayfire/bridge-commands.jsonl
```

Close-surface commands in that fixture are intentionally rejected by `policy`.
They belong in a future surface-control cell, not in the policy cache.

## `input`

Path:

```text
go/internal/input
```

Owns:

- current input actor context;
- set/clear semantics for actor uid.

Does not own:

- policy allow/deny projection;
- surface state;
- input event injection;
- compositor socket parsing.

`policy` may depend on this cell for actor context state. The dependency is
explicitly listed in `governance/ownership.toml`.

## Import Enforcement

Go internal package imports are checked against `governance/ownership.toml` by:

```bash
./harness/ci/check-depgraph.sh
```

Add a `go_package` entry before adding a new internal package. Add `may_import`
only when the dependency is an approved cell boundary.

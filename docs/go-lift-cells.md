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

## Import Enforcement

Go internal package imports are checked against `governance/ownership.toml` by:

```bash
./harness/ci/check-depgraph.sh
```

Add a `go_package` entry before adding a new internal package. Add `may_import`
only when the dependency is an approved cell boundary.


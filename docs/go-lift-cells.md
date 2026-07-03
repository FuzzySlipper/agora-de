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

## `shellui/surfaceactions`

Path:

```text
go/internal/shellui/surfaceactions
```

Owns:

- table-friendly surface action command decoding;
- close-surface and close-surfaces-by-owner command shape.

Does not own:

- policy projection;
- surface lifecycle projection;
- HTTP routing or static serving;
- launch/session association.

The package consumes `go/internal/wayfireproto` and extracts only action
commands from:

```text
compositor/protocol-fixtures/wayfire/bridge-commands.jsonl
```

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

## `session`

Path:

```text
go/internal/session
```

Owns:

- DE session tokens;
- session record lookup;
- token-scoped destroy semantics.

Does not own:

- process launch;
- surface association;
- compositor policy;
- shell HTTP auth.

## `launchlife`

Path:

```text
go/internal/launchlife
```

Owns:

- launch records;
- launch state transitions;
- association from launch records to mapped surface ids.

Does not own:

- session token creation or destruction;
- surface lifecycle projection;
- app catalog import;
- process execution.

`launchlife` may depend on `session` for token identity, but session does not
depend back on launch lifecycle behavior.

## `appcatalog`

Path:

```text
go/internal/appcatalog
```

Owns:

- launchable app metadata;
- conservative `.desktop` entry import;
- visible catalog entry projection.

Does not own:

- process launch;
- shell HTTP routes;
- surface association;
- policy decisions.

The first fixture lives at:

```text
harness/fixtures/appcatalog/example-browser.desktop
```

`shellui/catalog` may depend on this cell to expose catalog data through shell
APIs, but app catalog code does not depend on shell UI behavior.

## `shellui/catalog`

Path:

```text
go/internal/shellui/catalog
```

Owns:

- shell-facing catalog view projection.

Does not own:

- `.desktop` parsing;
- process launch;
- HTTP routing;
- policy or surface state.

This cell may depend on `appcatalog`, and only projects visible entries into
shell view data.

## `shellui/theme`

Path:

```text
go/internal/shellui/theme
```

Owns:

- theme manifest validation;
- `--agora-*` token validation;
- safe visual CSS rejection rules.

Does not own:

- arbitrary CSS support;
- theme marketplaces;
- layout-capable CSS;
- shell HTTP routing.

The first manifest fixture lives at:

```text
harness/fixtures/theme/agora-default-theme.json
```

Theme support is intentionally constrained to the token/sanitizer contract from
the successor lesson packet.

## `shellui/widgets`

Path:

```text
go/internal/shellui/widgets
```

Owns:

- packaged widget manifest validation;
- widget registry metadata;
- widget bus topic prefix shape.

Does not own:

- widget proxy HTTP serving;
- postMessage event handling;
- bus publication;
- shell layout decisions.

The first fixture lives at:

```text
harness/fixtures/widgets/clock-widget.json
```

Widget IDs and entrypoints are validated before any future proxy-serving code
can consume them.

## `shellui/staticserve`

Path:

```text
go/internal/shellui/staticserve
```

Owns:

- static asset path resolution;
- traversal and absolute-path rejection;
- default `/` to `index.html` behavior.

Does not own:

- HTTP route registration;
- theme manifest logic;
- widget proxy policy;
- shell state APIs.

This cell is the path-safety primitive future `/shell/`, theme asset, and widget
proxy serving code should share.

## `shellui/escalations`

Path:

```text
go/internal/shellui/escalations
```

Owns:

- shell-facing pending escalation projection;
- typed use of `go/internal/osboundary`.

Does not own:

- parsing predecessor log files;
- fallback escalation storage;
- audit socket transport;
- shell HTTP routing.

This cell must remain behind the typed `osboundary.EscalationClient` interface.

## `shellui/audittail`

Path:

```text
go/internal/shellui/audittail
```

Owns:

- shell-facing audit event projection;
- typed use of `go/internal/osboundary`.

Does not own:

- direct audit socket dialing;
- audit log parsing;
- event-bus broker behavior;
- shell HTTP routing.

This cell must remain behind the typed `osboundary.AuditClient` interface.

## `shellui/agenthealth`

Path:

```text
go/internal/shellui/agenthealth
```

Owns:

- shell-facing agent health projection;
- state counts for ready/busy/offline/unknown agents;
- typed use of `go/internal/osboundary`.

Does not own:

- agent supervisor internals;
- lifecycle topic subscription;
- event-bus broker behavior;
- shell HTTP routing.

This cell must remain behind the typed `osboundary.AgentClient` interface.

## Import Enforcement

Go internal package imports are checked against `governance/ownership.toml` by:

```bash
./harness/ci/check-depgraph.sh
```

Add a `go_package` entry before adding a new internal package. Add `may_import`
only when the dependency is an approved cell boundary.

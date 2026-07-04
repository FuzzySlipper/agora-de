# Go Lift Closeout Matrix

Phase 2 closes when lifted predecessor behavior lives in narrow Go cells with
machine-checked imports, focused tests, and no compatibility shim gravity.

The boundary checker reads:

```text
governance/ownership.toml
```

The full gate runs:

```bash
./harness/ci/check-all.sh
```

## Closeout Status

Phase 2 is ready to close from the Go lift side:

- every named lift cell has an ownership entry;
- every named lift cell has focused tests;
- shell-facing projections are split from HTTP routing;
- Wayfire protocol decoding remains fixture-backed;
- app catalog now proves a Go projection can feed a TypeScript successor
  feature through the protocol/package barrels;
- no predecessor workaround mode or runtime shim is preserved.

## Cell Matrix

| Concern | Owner package | Allowed internal dependencies | Focused tests | Non-ownership boundary | Remaining gap |
| --- | --- | --- | --- | --- | --- |
| Surface lifecycle | `go/internal/surfacetrack` | `wayfireproto` | `model_test.go` | Shell views, routes, policy, capture, launch/session association | Closed |
| Policy cache | `go/internal/policy` | `input`, `wayfireproto` | `cache_test.go` | Surface lifecycle, close commands, audit/escalation decisions | Closed |
| Input actor context | `go/internal/input` | none | `context_test.go` | Policy allow/deny projection, input injection, socket parsing | Closed |
| Session | `go/internal/session` | none | `store_test.go` | Process launch, surface association, shell HTTP auth | Closed |
| Launch lifecycle | `go/internal/launchlife` | `session` | `record_test.go` | Session token creation, app catalog import, process execution | Closed |
| Capture evidence | `go/internal/capture` | none | `evidence_test.go` | Readback transport, GLES/protocol capture implementation, shell routing | Closed |
| App catalog import | `go/internal/appcatalog` | none | `desktop_test.go` | Process launch, shell HTTP routes, policy decisions | Closed |
| App catalog shell view | `go/internal/shellui/catalog` | `appcatalog` | `view_test.go` | Desktop parsing, launch, HTTP routing | Closed |
| App catalog HTTP route | `go/internal/shellui/catalogroute` | `shellui/catalog` | `handler_test.go` | Catalog import/projection ownership | Closed |
| Theme | `go/internal/shellui/theme` | none | `manifest_test.go` | Live CSS injection, widget serving, shell routing | Closed |
| Widgets | `go/internal/shellui/widgets` | none | `manifest_test.go` | Proxy serving, bus publication, shell routing | Closed |
| Static serving | `go/internal/shellui/staticserve` | none | `resolver_test.go` | Theme/widget semantics, shell composition | Closed |
| Surface actions | `go/internal/shellui/surfaceactions` | `wayfireproto` | `actions_test.go` | Policy projection, lifecycle projection, HTTP routing | Closed |
| Surface shell view | `go/internal/shellui/surfaces` | `surfacetrack` | `view_test.go` | Wayfire decoding, projection mutation, HTTP routing | Closed |
| Surface HTTP route | `go/internal/shellui/surfaceroute` | `shellui/surfaces` | `handler_test.go` | Surface lifecycle projection and protocol decoding | Closed |
| Escalation projection | `go/internal/shellui/escalations` | `osboundary` | `view_test.go` | Agora OS authority, predecessor logs, routing | Closed |
| Audit-tail projection | `go/internal/shellui/audittail` | `osboundary` | `view_test.go` | Direct audit socket reads, predecessor logs, routing | Closed |
| Agent-health projection | `go/internal/shellui/agenthealth` | `osboundary` | `view_test.go` | Supervisor internals, predecessor logs, routing | Closed |
| Wayfire protocol decoding | `go/internal/wayfireproto` | none | `messages_test.go` | Policy, lifecycle, capture, shell routes | Closed |

## Vertical Evidence

The first successor vertical uses:

- Go catalog import and projection: `go/internal/appcatalog`,
  `go/internal/shellui/catalog`, `go/internal/shellui/catalogroute`;
- Rust-owned generated contract shape: `CatalogApp` and
  `CatalogAppsResponse`;
- TypeScript route decoding: `@agora-de/transport`;
- TypeScript async state: `@agora-de/store`;
- app launcher view model: `@agora-de/feature-app-launcher`;
- fixture assertion: `assertAppCatalogVerticalFixture`.

That path proves a lifted Go projection can feed a UI successor surface without
pulling predecessor internals or runtime fallbacks into the new repo.

## Shim Audit

No Go lift cell contains a legacy fallback mode, predecessor log-file adapter,
or runtime compatibility shim. The `osboundary` package remains fail-closed
until agora-os exposes typed service APIs.

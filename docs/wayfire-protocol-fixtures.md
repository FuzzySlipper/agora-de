# Wayfire Protocol Fixtures

The Wayfire backend speaks newline-delimited JSON over a Unix socket. The
successor keeps fixture files for both directions before lifting the predecessor
plugin and bridge implementation.

## Fixtures

```text
compositor/protocol-fixtures/wayfire/plugin-events.jsonl
compositor/protocol-fixtures/wayfire/layout-state-events.jsonl
compositor/protocol-fixtures/wayfire/layout-action-events.jsonl
compositor/protocol-fixtures/wayfire/bridge-commands.jsonl
compositor/protocol-fixtures/wayfire/layout-authority-probe-4267.json
```

`plugin-events.jsonl` covers messages emitted by the compositor plugin:

- surface mapped;
- surface focused;
- input denied.

`layout-state-events.jsonl` covers the authoritative backend layout snapshot
emitted by the Wayfire plugin after it has compositor-owned post-layout
geometry, zone/workspace state, focus state, and surface order.

`layout-action-events.jsonl` covers compositor plugin responses for direct
layout placement commands.

`bridge-commands.jsonl` covers messages sent by the Go bridge:

- policy replace;
- policy upsert;
- policy remove;
- input context set/clear;
- close surface;
- close surfaces by owner uid;
- place surface.

`layout-authority-probe-4267.json` records the Wayfire 0.10.1 API surface used
to decide whether the next layout-authority proof can proceed through direct
Wayfire plugin APIs. It is not socket protocol. It is a checked evidence packet
for the backend decision series.

## Consumers

The Go protocol package lives at:

```text
go/internal/wayfireproto
```

The package tests decode the JSONL fixtures. The compositor harness also checks
that every fixture line is valid JSON and uses a known message type:

```bash
./harness/ci/check-compositor.sh
```

The Go surface event kind strings are checked against the generated TypeScript
contract so `wayfireproto` does not drift from the Rust-owned protocol
vocabulary.

The layout-authority probe fixture is checked by the compositor harness for
schema, task id, required capability entries, and the next proof task id.

When the C++ plugin lands, its protocol smoke tests should use these same
fixture shapes or a generated equivalent.

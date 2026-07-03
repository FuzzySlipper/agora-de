# Wayfire Protocol Fixtures

The Wayfire backend speaks newline-delimited JSON over a Unix socket. The
successor keeps fixture files for both directions before lifting the predecessor
plugin and bridge implementation.

## Fixtures

```text
compositor/protocol-fixtures/wayfire/plugin-events.jsonl
compositor/protocol-fixtures/wayfire/bridge-commands.jsonl
```

`plugin-events.jsonl` covers messages emitted by the compositor plugin:

- surface mapped;
- surface focused;
- input denied.

`bridge-commands.jsonl` covers messages sent by the Go bridge:

- policy replace;
- policy upsert;
- policy remove;
- input context set/clear;
- close surface;
- close surfaces by owner uid.

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

When the C++ plugin lands, its protocol smoke tests should use these same
fixture shapes or a generated equivalent.

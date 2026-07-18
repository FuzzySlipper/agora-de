# Settings Module Authoring Guide

Settings v1 is a build-time registry of reviewed first-party modules. It is not
a stable third-party plugin ABI. Adding a module changes product code and must
pass the same ownership, contract, security, and live-evidence gates as the
existing Diagnostics module.

Read [Settings v1 Architecture](settings-v1-architecture.md) first.

## Before adding a module

A request belongs in Settings when it configures durable or inspectable desktop
behavior, has a bounded authority, and benefits from load/edit/validate/apply/
reset/default semantics.

Keep it outside Settings when it is:

- a one-shot user action better placed in the launcher or taskbar;
- governance, isolation, audit, or agent-supervision authority owned by
  `agora-os`;
- an arbitrary command/file editor;
- a transient application workflow rather than desktop configuration;
- unsupported by a live authority and would require a fake preference bit.

If ownership is unclear, stop for planner review before adding a package.

## Naming and routing

Choose one stable lowercase module ID containing only `a-z`, `0-9`, and `-`.
Use it for the generated manifest `id` and `route`. Titles and categories may
change; IDs may not.

The gateway routes are fixed:

```text
GET  /api/settings/catalog
GET  /api/settings/modules/<id>/load
POST /api/settings/modules/<id>/validate
POST /api/settings/modules/<id>/apply
POST /api/settings/modules/<id>/reset
POST /api/settings/modules/<id>/restore_defaults
```

The host deep link is
`/shell/dist/desktop/?surface=settings&module=<id>`. Do not add a host route,
navigation switch, or generic setting-name endpoint.

## 1. Rust contract and authority seam

Add the module manifest vocabulary and explicitly versioned module state,
draft, validation, and result types to `protocol-settings`. Common envelopes
may be reused, but a module payload must be a named type; never add
`payload: JSON`, `map<string, any>`, or arbitrary keys.

Required contract coverage:

- stable wire names and non-zero contract version;
- all required fields and strict unknown-field behavior;
- catalog and module-state round trip;
- a complete apply request with base revision;
- validation issues and relevant typed errors;
- unsupported newer module/version behavior.

If the feature owns durable validation, persistence, or a safety state machine,
implement that in a Rust state/service crate registered in
`governance/ownership.toml`.

## 2. Generate language borders

Extend `protocol-codegen`; do not edit generated TypeScript or Go output.

```bash
cargo run --manifest-path de-rs/Cargo.toml -p protocol-codegen -- \
  --write ts/packages/protocol/src/generated/contracts.ts

cargo run --manifest-path de-rs/Cargo.toml -p protocol-codegen -- \
  --write-go-settings go/internal/settingsprotocol/generated.go
```

Re-export TypeScript types through `@agora-de/protocol`. Add canonical fixtures
under `harness/fixtures/settings/` and generated-border round-trip tests.

## 3. Optional Go integration adapter

Use Go only to carry a proven daemon/service integration. A module adapter must
provide the structural `settingsregistry.Module` seam:

```go
type Module interface {
    Manifest() settingsprotocol.SettingsModuleManifest
    Availability(context.Context) settingsprotocol.SettingsModuleAvailability
    Handler() http.Handler
}
```

The handler decodes module-specific generated requests. It receives only the
bounded operation path (`/state`, `/validate`, `/apply`, `/reset`, or
`/defaults`). Never accept commands, paths, service names, compositor keys, or
generic payloads from the client.

Register the adapter once in the shell composition. The registry supplies
catalog/navigation data; no other module or host switch changes.

Adapters must:

- honor request cancellation and bounded timeouts;
- report availability without preventing other modules from loading;
- reject stale revisions and overlapping mutation where applicable;
- return authoritative state after apply;
- preserve/restore the prior state on partial failure when the authority can
  mutate more than one value;
- expose typed errors rather than raw command structure or governance logs.

## 4. Independent TypeScript page

Create `ts/packages/feature-settings-<id>` with one root barrel and register it
in `governance/ownership.toml`. It may consume generated protocol/domain/shared
presentation packages allowed by ownership. It may not import
`feature-settings-host` or another settings feature.

Export a `SettingsPageRegistration` whose `uiEntryPoint` matches the generated
manifest. Its `load()` returns a page definition lazily. The shell composition
root is the only production package allowed to collect page registrations.

Use the host lifecycle helpers for active/draft/dirty/revision state. TypeScript
does not persist settings, execute commands, talk to Wayland, or own rollback
timers.

## 5. Test with the authoring kit

`@agora-de/settings-testing-fixtures` contains a non-production fixture module
and reusable lifecycle assertions. It proves:

- manifest/registration metadata matching;
- catalog search and stable deep-link resolution;
- lazy page loading and independent load failure;
- edit, reset, defaults, apply, and revision transitions;
- unavailable/error presentation;
- rejection when required lifecycle capability is missing.

The depgraph gate has a representative sibling-feature violation and must
reject it. It also verifies that the fixture package is not imported by the
production shell or shell-asset generator.

## Security review questions

- What process owns the authoritative value?
- Can every client-controlled string be represented as an enum, ID, or bounded
  value instead?
- Which fixed operations may the Go adapter perform?
- What happens if the client disconnects, times out, retries, or submits a
  stale revision?
- Can an apply leave the desktop unusable? If so, where is the authority-owned
  test/confirmation/rollback state machine?
- What state is persisted, where, at what version, and with what atomicity?
- Does any response disclose secrets, arbitrary logs, user content, paths, or
  window/clipboard data?
- Is a restart actually required, and is the target allowlisted?

## Completion checklist

- [ ] Rust-owned module contract and validation exist.
- [ ] Generated TypeScript and Go borders are current.
- [ ] Canonical fixtures round-trip and reject missing/unknown fields.
- [ ] New crates/packages are registered in `governance/ownership.toml`.
- [ ] Adapter availability, timeout, stale, validation, failure, and recovery
      paths are covered.
- [ ] TypeScript page is independent and lazy registered.
- [ ] Keyboard, focus, labels, validation summary, and unavailable state are
      tested.
- [ ] The fixture never appears in a production catalog or shell build.
- [ ] `check-contracts.sh`, `check-rust.sh`, `check-go.sh`, `check-ts.sh`, and
      `check-depgraph.sh` pass.
- [ ] Installed-session behavior has capture-visible evidence.

# Settings v1 Architecture

Status: accepted for implementation  
Decision task: Den `#5912`  
Campaign: Den `#5911`

## Context

Agora DE already has several configuration seams, but they do not yet form a
control center:

- `shell-settings` launches a 760x520 layer-shell popup whose only control is
  the diagnostic overlay.
- `GET/POST /api/settings` is a hand-written, diagnostic-specific Go handler
  that calls `systemctl --user` for `agora-de-shell-overlay.service`.
- layout state and mutations use separate `/api/layout` routes backed by the
  compositor bridge and `compositorctl`.
- the shell has a validated theme catalog and token sanitizer, but theme
  selection is not presented as a settings module.
- Agora-managed keybindings are generated from repository-owned inputs rather
  than edited through a durable settings contract.
- output discovery currently exists as compositor/CLI evidence; there is no
  safe display mutation, confirmation, rollback, or persistence authority.

The first release needs one coherent application without turning these seams
into a generic configuration bag. It also needs a path for later first-party
configuration features to land independently.

## Decision

Settings v1 is a normal XDG toplevel application composed from a
**build-time, first-party module registry**. The registry is generated from
Rust-owned manifests and compiled into the Go gateway and TypeScript
projection. It is not a runtime plugin ABI.

Each module owns a bounded contract, backend adapter, and TypeScript page. The
host owns navigation, search, deep links, lifecycle chrome, and failure
isolation. It does not switch over module-specific fields.

Rust remains the source of truth for new settings contracts, validation
vocabulary, revisions, persistence policy, and safety-critical authorities.
Go carries the existing system-service and gateway behavior while it is lifted
behind generated contracts. TypeScript owns composition and projection only.

## Module manifest

Every production module has a generated `SettingsModuleManifest` with:

| Field | Meaning |
| --- | --- |
| `id` | Stable lowercase identifier; never derived from the display title. |
| `category` | Stable navigation group such as `hardware`, `personal`, or `system`. |
| `title` | User-facing module name. |
| `summary` | One-sentence navigation/search description. |
| `icon` | Agora icon token, not an arbitrary path or URL. |
| `route` | Stable page key used by the host deep link. |
| `searchTerms` | Bounded generated aliases; no runtime HTML. |
| `capabilities` | Typed operations supported by the module. |
| `backendAdapter` | Stable gateway adapter ID. |
| `uiEntryPoint` | Build-time TypeScript registration ID. |
| `contractVersion` | Module payload version understood by both sides. |

The canonical v1 deep link is the Settings launch URL with a `module` query,
for example `/shell/dist/desktop/?surface=settings&module=diagnostics`. The
module ID is the durable part; categories, ordering, and titles may change.

Unknown modules are not loaded. A known module with an unavailable backend is
listed with a typed reason so users can understand and retry it.

## Common lifecycle

The common envelope is typed and revisioned; module payloads remain separate,
versioned types rather than an untyped JSON object.

1. **Catalog** returns manifests plus availability and capabilities.
2. **Load** returns authoritative active state, its revision, and optional
   defaults. The host creates a local draft from that state.
3. **Edit** changes only the local draft and marks the page dirty.
4. **Validate** performs local field checks where possible and authoritative
   validation before mutation.
5. **Apply** sends the complete bounded draft with its base revision. Success
   returns a new authoritative revision and any `restart_required` outcome.
6. **Reset** discards the draft and reloads active state; it is not a backend
   mutation.
7. **Restore defaults** asks the module authority for its typed default draft;
   the user must still Apply unless the module contract explicitly documents an
   immediate safety action.
8. **Unavailable/read-only** preserves navigation and explanatory state while
   disabling unsupported operations.

The host warns before discarding a dirty draft. It never treats an HTTP success
alone as committed state; after Apply it renders the authoritative result.

## Errors and concurrency

The shared error vocabulary is:

- `invalid_request`: malformed envelope or unsupported contract version;
- `validation_failed`: one or more typed field issues;
- `stale_revision`: active state changed after the draft was loaded;
- `unsupported`: the module or requested capability is not supported;
- `unavailable`: the adapter or required service is not currently usable;
- `timeout`: the bounded adapter deadline elapsed;
- `apply_failed`: mutation failed without committing the proposed state;
- `rollback_failed`: a safety rollback could not restore the prior state;
- `restart_required`: apply committed, but a named bounded component must be
  restarted through its approved integration path.

Revisions are opaque unsigned integers scoped to one module instance. An Apply
must include the loaded revision. The gateway serializes mutations per module;
authorities reject stale or overlapping work. A transport retry cannot assume
that a mutation failed, so clients reload before offering another Apply.

## Authority, privilege, and persistence

- Rust authorities validate durable values and own persistence formats.
- Go adapters may call only allowlisted service operations or established
  bridge APIs. They do not accept commands, paths, unit names, or generic keys
  from TypeScript.
- TypeScript never edits TOML/INI/JSON product configuration, invokes
  `systemctl`, or talks directly to Wayland.
- Durable writes use versioned formats and atomic replacement in an
  Agora-owned user configuration directory. Only a confirmed display topology
  becomes persistent.
- Live Apply is preferred when the authority can prove the result. A restart is
  explicit in the typed result and limited to an allowlisted component.
- Display configuration uses standard output-management authority with
  protocol test/apply, an authority-owned confirmation lease, and automatic
  rollback. `wayfire.ini` and `wlr-randr` are not product APIs.
- Governance logs remain evidence, never settings state.

## Package ownership

The foundation adds only ownership-map registered packages:

| Layer | Package | Responsibility |
| --- | --- | --- |
| Rust protocol | `protocol-settings` | Manifests, lifecycle envelopes, errors, revisions, diagnostic payload. |
| Rust codegen | `protocol-codegen` | Generated TypeScript and Go settings borders. |
| Go generated border | `go/internal/settingsprotocol` | Generated wire structs/constants only. |
| Go registry | `go/internal/settingsregistry` | Build-time adapter registration, catalog, dispatch, timeout/failure isolation. |
| Go route | `go/internal/shellui/settingsroute` | HTTP decoding/encoding and origin/method policy. |
| Go adapter | `go/internal/shellui/settingsdiagnostics` | Allowlisted overlay service integration. |
| TypeScript host | `@agora-de/feature-settings-host` | Navigation, search, route, lifecycle shell, generic errors. |
| TypeScript module | `@agora-de/feature-settings-diagnostics` | Diagnostics page projection only. |
| TypeScript fixture | `@agora-de/settings-testing-fixtures` | Non-production module and contract/lifecycle helpers. |

Feature modules may import protocol, platform, transport, domain, store,
renderer, components, and theme lanes permitted by governance. They may not
import sibling settings features. The host may import first-party module entry
points solely as the composition root; feature pages do not import the host.

## Dependency flow

```text
protocol-settings (Rust source of truth)
          |
          v
 protocol-codegen --------------------+
          |                            |
          v                            v
generated Go border           generated TypeScript border
          |                            |
          v                            v
settings registry + adapters      module pages
          |                            |
          +---------- HTTP ------------+
                          |
                          v
                 settings host toplevel
```

Backend adapters depend inward on the generated border. The registry knows
adapters but not module-specific UI. The host knows generated manifests and UI
entry points but not module backend implementation details.

## V1 modules

1. **Displays**: output discovery, safe topology changes, confirmation,
   rollback, persistence, startup, and hotplug reconciliation.
2. **Window Management**: existing layout/rule/workspace/gap authority.
3. **Appearance**: validated theme catalog, preview, selection, and persistence.
4. **Shortcuts**: managed binding read/write, validation, conflict detection,
   regeneration, and approved reload.
5. **Diagnostics & About**: overlay control, versions, bounded health, recovery
   guidance, and a privacy-reviewed support bundle.

Network, Bluetooth, audio, power, users, printers, application defaults,
arbitrary third-party modules, and Plasma-level breadth are not v1 scope.

## End-to-end example: Diagnostics overlay

1. `protocol-settings` declares the `diagnostics` manifest and typed overlay
   active/default state.
2. Codegen emits the Go and TypeScript vocabulary.
3. `settingsdiagnostics` reads the allowlisted
   `agora-de-shell-overlay.service` through the configured `systemctl --user`
   binary and reports typed availability.
4. `settingsregistry` registers that adapter at build time and isolates its
   load/apply deadline from other modules.
5. `settingsroute` exposes catalog, load, and apply operations; there is no
   generic setting-name request.
6. The host finds the generated manifest, routes `module=diagnostics` to the
   diagnostics entry point, and renders the active/draft state.
7. Apply sends the base revision and the complete typed overlay draft. The
   adapter runs only enable/disable for its fixed unit, then returns fresh
   authoritative state.

If the service is absent, Diagnostics remains navigable and reports
`unavailable`; Displays and every other module continue to load.

## Migration

The existing diagnostic toggle moves first:

1. Introduce generated contracts, registry, route, and Diagnostics adapter.
2. Keep a temporary read-only redirect from the old Settings URL while the new
   catalog route is installed; do not keep a second runtime mode.
3. Change the catalog launcher from layer-shell `popup` to one normal `toplevel`
   Settings surface and preserve one-instance activation.
4. Remove the old `/api/settings` handler after its tests and live behavior are
   represented through the Diagnostics module routes.
5. Remove popup-only close/position assumptions after live toplevel proof.

## Recovery behavior

- Module load failure is isolated and retryable from that page.
- Stale edits are never merged implicitly; reload shows authoritative state.
- Failed Apply preserves or reloads the previous active state and retains a
  user-inspectable draft only when retry is safe.
- Restart-required success records committed state before requesting the
  bounded restart.
- Corrupt persistence falls back to documented defaults and exposes a typed
  diagnostic; it is not silently rewritten until the user confirms a change.
- Display rollback is timed by Rust authority, not the Settings window. Closing
  or crashing the UI cannot strand an unconfirmed topology.

The supported contribution path and review checklist are documented in the
[Settings Module Authoring Guide](settings-module-authoring.md).

## Rejected alternatives

- A larger `/api/settings` map: hides ownership and becomes an unversioned
  cross-language protocol.
- Hand-written Go/TypeScript mirrors: drift from Rust authority.
- Runtime module discovery: creates an unsupported plugin/security ABI in v1.
- TypeScript feature-to-feature imports: couples release order and breaks
  independent ownership.
- Raw compositor config edits or command output parsing: bypasses live
  authority, capability checks, and rollback.
- Another shell runtime or layer-shell popup: masks launch/platform defects and
  prevents normal application behavior.

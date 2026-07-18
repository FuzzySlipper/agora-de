# Agora DE Architecture

`agora-de` succeeds the desktop concern from `agora-os`: compositor mediation,
shell, chrome, webview hosting, app launch, browser gateway, and live visual
evidence. Governance, isolation, audit authority, and agent supervision stay in
`agora-os`.

## Boundary Posture

The repos meet at a small versioned protocol boundary. `agora-os` owns
governance authority; `agora-de` owns desktop state, compositor mediation, shell
projection, chrome, and evidence.

No repo imports the other's internals. If a value crosses the boundary, it must
live in a protocol crate/module and be generated or conformance-tested.

## Language Roles

Rust:
- authoritative DE value types;
- protocol source of truth for new contracts;
- evidence classification;
- compositor backend interfaces;
- future Smithay/custom-compositor experiments.

Go:
- lifted daemon behavior that already has working tests;
- bridge/socket/web gateway operations during the successor lift;
- CLI surfaces while parity is being re-established.

TypeScript:
- desktop shell and operator console;
- feature stores, projections, and renderer/customizer surfaces;
- generated protocol consumption only.

## Compositor Posture

Wayfire is the initial backend because it is the proven predecessor path. It is
not the architecture. The backend contract lands in Rust, with Wayfire as one
implementation and standard-protocol / Smithay spikes kept separate until they
earn product status.

## Chrome Posture

Native chrome is product source in `chrome/`, not deployment glue. Webviews are
appropriate for content and customization surfaces; panel/dock reliability is
decided by spikes and evidence, not fallback mode accumulation.

## Evidence Posture

Deterministic tests are necessary but not sufficient for user-visible UI work.
Live VM/capture evidence must travel with the successor before cutover.

## Settings Posture

Settings is a normal XDG toplevel composed from a build-time registry of
first-party modules. Rust owns the lifecycle contracts and durable validation,
Go owns lifted integration adapters, and TypeScript owns host/module
projection. The detailed decision, lifecycle, ownership map, migration, and
recovery rules are recorded in [Settings v1 Architecture](../docs/settings-v1-architecture.md).

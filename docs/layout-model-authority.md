# Layout Model Authority

Agora-de layout authority is split deliberately:

- Rust `layout-model` owns backend-independent state and command semantics.
- Backend adapters supply compositor facts: surface lifecycle, post-layout
  geometry, focus result, workspace result, and support/unsupported outcomes.
- Go bridge code may transport, cache, and expose those facts, but should not
  invent higher-level layout semantics when the Rust model has a corresponding
  rule.
- TypeScript shell code projects layout state and asks for actions; it should
  not own command result classification.

## Rust-Owned Semantics

The Rust layout model owns:

- layout mode and revision advancement;
- workspace surface order and focus order;
- surface participation: tiled, floating, transient;
- accepted versus rejected command status;
- stable error classes: `invalid_request`, `surface_not_found`,
  `surface_stale`, and `backend_unsupported`;
- refusal to accept zone or tile commands without backend-provided geometry.

The model treats geometry as a compositor fact. If an adapter cannot provide
truthful post-layout geometry for a command, it must return
`backend_unsupported` rather than fabricating placement from screenshots,
shell state, or key bindings.

## Adapter Boundary

Wayfire, Smithay, or another backend adapter may:

- map backend surface ids to contract surface ids;
- execute direct compositor commands;
- report post-layout geometry and visibility;
- report focus/workspace command outcomes;
- classify unsupported backend operations.

Adapters should not own:

- shell UI policy;
- theme or launcher behavior;
- product-specific layout heuristics beyond the command result they execute;
- governance or audit log reads.

The shared fixture
`compositor/protocol-fixtures/layout-model/command-semantics.json` is the
backend-neutral behavior target for command implementations. The fixture is
checked by `./harness/ci/check-contracts.sh`.

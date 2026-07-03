# Contract Governance

Cross-language contracts are borders. Changing a contract means changing how
Rust, Go, TypeScript, and the `agora-os` boundary understand one another.

## Source Of Truth

Rust protocol crates under `de-rs/crates/protocol/*` are the source of truth for
new DE contracts. TypeScript generated output is committed for worker
convenience, but it is not hand-owned.

Current generated TypeScript output:

```text
ts/packages/protocol/src/generated/contracts.ts
```

Current generated evidence vocabulary includes `CaptureClassification`, which
mirrors the Go capture evidence ladder terms:

- `insufficient_mapped_only`
- `frame_presented`
- `capture_visible`
- `blank_capture_failure`
- `not_visible`

Evidence wire names are owned by `de-rs/crates/protocol/protocol-evidence`;
codegen emits the TypeScript unions from that Rust vocabulary.

Compositor surface event wire names are owned by
`de-rs/crates/protocol/protocol-compositor` and emitted the same way.

## Regeneration

```bash
cargo run --manifest-path de-rs/Cargo.toml -p protocol-codegen -- \
  --write ts/packages/protocol/src/generated/contracts.ts
```

Check drift without writing:

```bash
./harness/ci/check-contracts.sh
```

The full gate runs the same drift check:

```bash
./harness/ci/check-all.sh
```

## Rules

- Do not hand-edit files under `ts/packages/protocol/src/generated/`.
- Commit Rust protocol changes and generated TypeScript changes together.
- TypeScript imports protocol contracts through `@agora-de/protocol` only.
- Go/C++ protocol fixtures must eventually be checked against the same source
  of truth or an explicitly documented conformance adapter.

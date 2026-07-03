# agora-de

Successor desktop-environment repository for the Agora GUI/DE concern.

This repo is intentionally not a compatibility shim for `../agora-os`. The old
DE code is predecessor evidence: behavior, tests, operational scars, and
selection pressure. New implementation work lands here only after it can live
inside the successor boundaries.

## Current Posture

- No legacy runtime fallback.
- No deployment bridge to the old shell.
- No preservation of the old shell surface-mode matrix.
- No hand-written TypeScript protocol mirrors.
- Deploy only after the repo's own VM/live/capture evidence is solid.

## Layout

```text
docs/          successor briefs and durable design notes
governance/    machine-readable ownership and boundary rules
harness/       CI, depgraph checks, fixtures, and live evidence harnesses
de-rs/         Rust authority, protocol, evidence, and compositor-backend crates
go/            lifted-and-split daemons from the predecessor
ts/            TypeScript shell/customizer workspace
compositor/    Wayfire backend, protocol fixtures, and compositor spikes
chrome/        native chrome and webview-host spikes
deploy/        final productization only, not product source
```

## First Commands

```bash
cd ts && npm install && cd ..
./harness/ci/check-all.sh
./harness/ci/check-rust.sh
./harness/ci/check-go.sh
./harness/ci/check-ts.sh
./harness/ci/check-depgraph.sh
```

The scaffold is deliberately small. Its job is to make the desired boundaries
compile and fail mechanically before feature code starts arriving. TypeScript
uses the local compiler installed under `ts/node_modules`.

## References

- [Successor brief](docs/successor-brief.md)
- [Successor lesson packet](docs/successor-lesson-packet.md)
- [Architecture](governance/architecture.md)
- [Contract governance](docs/contract-governance.md)
- [Ownership](governance/ownership.toml)

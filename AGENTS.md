# Agora DE Agent Guide

This repo is a successor implementation. The predecessor `../agora-os` GUI/DE
code is evidence only: read it for behavior, fixtures, contracts, and platform
lessons, but do not preserve legacy runtime modes or compatibility fallbacks.

## Architecture Soul

Rust owns new authority and contracts. Go carries proven daemon behavior while
it is lifted and split. TypeScript owns shell expression and projection.

Generated contracts define language borders. Hand-written cross-language
protocol mirrors are forbidden.

## Hard Rules

- Do not import `../agora-os` internals.
- Do not add shims for the old running DE.
- Do not put product code under `deploy/`.
- Do not add a new shell runtime mode to route around a platform defect.
- Do not read governance logs as a product API.
- Use `go/internal/osboundary` for governance-facing Go contracts.
- Do not hand-edit generated protocol output once generation is active.
- Regenerate protocol output with `protocol-codegen`; do not patch generated
  TypeScript by hand.
- Do not let TypeScript feature libraries import sibling feature libraries.
- Do not bypass `governance/ownership.toml` when adding crates or packages.

When a task seems to require breaking a boundary, stop and ask for planner
review.

## Local Checks

```bash
./harness/ci/check-all.sh
```

Focused checks:

```bash
./harness/ci/check-rust.sh
./harness/ci/check-go.sh
./harness/ci/check-ts.sh
./harness/ci/check-depgraph.sh
./harness/ci/check-contracts.sh
./harness/ci/check-compositor.sh
```

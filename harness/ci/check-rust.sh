#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

cargo check --manifest-path "$ROOT/de-rs/Cargo.toml" --workspace


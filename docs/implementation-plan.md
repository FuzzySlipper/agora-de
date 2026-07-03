# Agora DE Implementation Plan

This plan assumes no current dependency on the predecessor DE runtime. The
successor can wait to deploy until its own stack is coherent and evidenced.

## Phase 0: Rails

- Keep this scaffold green with `./harness/ci/check-all.sh`.
- Install TypeScript tooling with `cd ts && npm install` before running the full
  local gate on a fresh checkout.
- Treat `governance/ownership.toml` as the machine-readable assignment map.
- Add crates/packages only with ownership entries and boundary checks.

## Phase 1: Boundary Contracts

- Define the typed `agora-os` boundary for agent/audit/escalation data.
- Keep the Go boundary package fail-closed until `agora-os` exposes a typed
  service endpoint; do not bridge through predecessor log files.
- Define the compositor backend API in Rust.
- Keep compositor capability claims in the checked backend matrix before
  promoting compositor spikes to product work.
- Stand up protocol generation before TypeScript consumes real contracts.

## Phase 2: Lift And Split

- Move predecessor Go daemon behavior into `go/internal/*` cells.
- Treat `go/internal/surfacetrack` as the first exemplar lift cell: narrow
  ownership, fixture-backed behavior, checked imports.
- Keep `go/internal/policy` limited to policy-cache projection and input actor
  context from `go/internal/input`; surface close/control commands land
  in `go/internal/shellui/surfaceactions`.
- Keep launch/session split: `session` owns token lifecycle, `launchlife` owns
  launch records and surface association state.
- Keep capture evidence classification separate from compositor readback
  transport.
- Keep app catalog import separate from process launch and shell HTTP routing.
- Keep shell theme support limited to manifest and `--agora-*` token sanitizer
  behavior until live UI evidence requires more.
- Keep widget manifest/registry validation separate from proxy serving and bus
  publication.
- Keep static asset path safety in `shellui/staticserve` before adding shell,
  theme, or widget proxy routes.
- Keep escalation UI projection behind `osboundary`; no log-file fallback.
- Keep audit-tail UI projection behind `osboundary`; no direct audit socket.
- Keep agent-health UI projection behind `osboundary`; no supervisor internals.
- Keep predecessor tests as selection pressure, but delete workaround modes.
- Preserve socket protocol fixtures for Wayfire conformance and keep
  `go/internal/wayfireproto` fixture-backed.

## Phase 3: UI Successor

- Build desktop shell and operator console as two thin apps over shared feature
  libraries.
- Consume generated contracts only.
- Require `AsyncState<T>` for async store state and classified transport errors.

## Phase 4: Compositor And Chrome Decisions

- Keep Wayfire as the initial backend.
- Run the standard-protocol probe before changing compositor direction.
- Run the GTK4/WebKitGTK layer-shell spike before choosing webview chrome.
- Promote native chrome only as product source under `chrome/`.

## Phase 5: Evidence And Deploy

- Port VM and phase live checks before cutover.
- Close user-visible shell claims only with inspected evidence packets.
- Use `deploy/` only for final productization, not source code.

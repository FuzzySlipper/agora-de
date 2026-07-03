# Agora DE Implementation Plan

This plan assumes no current dependency on the predecessor DE runtime. The
successor can wait to deploy until its own stack is coherent and evidenced.

## Phase 0: Rails

- Keep this scaffold green with `./harness/ci/check-all.sh`.
- Treat `governance/ownership.toml` as the machine-readable assignment map.
- Add crates/packages only with ownership entries and boundary checks.

## Phase 1: Boundary Contracts

- Define the typed `agora-os` boundary for agent/audit/escalation data.
- Define the compositor backend API in Rust.
- Stand up protocol generation before TypeScript consumes real contracts.

## Phase 2: Lift And Split

- Move predecessor Go daemon behavior into `go/internal/*` cells.
- Keep predecessor tests as selection pressure, but delete workaround modes.
- Preserve socket protocol fixtures for Wayfire conformance.

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


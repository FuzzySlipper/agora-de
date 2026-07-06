# Smithay Layout Authority Spike

Status: parked after Den tasks 4268 and 4269 passed the Wayfire layout state
and command proofs.

This is a comparison spike plan, not permission to start a compositor rewrite.
The current backend remains Wayfire while it stays inside the Den 4251 churn
budget and avoids synthetic shortcuts, screenshot-derived geometry, and shell
policy in the backend.

## Primary Sources

- Smithay README: <https://github.com/Smithay/smithay>
- Smithay docs.rs: <https://docs.rs/smithay>
- Smithay getting started guide:
  <https://github.com/Smithay/smithay/blob/master/GETTING_STARTED.md>
- Smithay Anvil README:
  <https://github.com/Smithay/smithay/blob/master/anvil/README.md>

Current reading:

- Smithay is a low-level compositor building-block library, not a complete
  desktop compositor.
- Window management and drawing logic remain agora-de responsibilities.
- `smallvil` is the smallest learning-oriented starting point.
- `anvil` is the larger Smithay feature testbed to study when the spike needs
  more complete backend examples.

## Start Trigger

Start Smithay implementation only if the Wayfire path hits one of these
conditions:

- layout actions require synthetic keyboard shortcuts;
- geometry must be inferred from shell state, output capture, or screenshots;
- the backend depends on unexported `simple-tile` internals;
- the Wayfire plugin grows into shell UI, launcher, governance, or theme policy;
- the Wayfire path exceeds the Den 4251 churn budget.

As of Den 4268 and Den 4269, the trigger is not active. Wayfire produced
capture-visible structured placement, CLI command results, and backend-reported
layout state through the installed den-k8 session.

## Minimal Deliverables

1. Host two real native `xdg-shell` clients on den-k8.
2. Own primary/secondary tiled zone geometry inside compositor state.
3. Apply `layout-model` command semantics for assign-zone or tile plus
   move-resize.
4. Report workspace surface order and focus order.
5. Classify stale, missing, invalid, and backend-unsupported command results.
6. Produce capture-visible evidence of non-overlap and agent-identifiable
   surfaces.

## Non-Goals

- Shell UI, launcher, dock, panel, or theme work.
- Governance or audit log integration.
- Webview wrapping around native clients.
- Niri-like column history or broader product window-management design.
- A full desktop replacement.

## Den 4251 Criteria Mapping

| Criterion | Smithay Proof Action |
| --- | --- |
| Authoritative post-layout geometry | Own xdg-shell placement and emit geometry after configure/commit. |
| Commandable layout actions | Implement assign-zone or tile plus move-resize against `layout-model` semantics. |
| Stable workspace/focus order | Keep explicit `surface_order` and `focus_order` in compositor state. |
| Cleanup/stale classification | Remove or mark unmapped surfaces and return stable model error classes. |
| Capture-visible annotations | Add only minimal compositor-owned bounds/labels if ordinary surface evidence is insufficient. |

## Validation

Local validation:

```bash
cargo test --manifest-path de-rs/Cargo.toml -p smithay-spike
cargo test --manifest-path de-rs/Cargo.toml -p layout-model
./harness/ci/check-compositor.sh
```

Live validation, if the trigger becomes active:

1. Run the spike nested first, not as the installed den-k8 compositor.
2. Launch Alacritty and foot against the spike Wayland socket.
3. Apply the shared layout command fixture behavior from
   `compositor/protocol-fixtures/layout-model/command-semantics.json`.
4. Capture output evidence proving non-overlap and visible client identity.
5. Compare code size and ownership against the Wayfire plugin path before
   promoting Smithay.

## Stop/Go Criteria

Go only if Smithay hosts real native clients and satisfies the same layout
contract with less backend churn than the Wayfire proof.

Stop if the spike requires a full shell rewrite, cannot host real native
clients on den-k8, cannot produce capture-visible evidence, or starts absorbing
launcher/theme/governance concerns.

The machine-readable companion is
`compositor/protocol-fixtures/smithay/layout-authority-spike-4271.json`.

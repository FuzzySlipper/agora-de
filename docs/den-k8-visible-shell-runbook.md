# den-k8 Visible Shell Runbook

This runbook covers the agora-de user-service shell path on den-k8 class hosts.
It intentionally tests against the installed Wayfire/compositor service and does
not create VMs.

## Current Baseline

The successor shell uses user units:

```text
agora-de-shellui.service
agora-de-shell-background.service
agora-de-shell-panel.service
```

The predecessor visible shell stack is root-owned systemd state:

```text
agora-shell-panel.service
event-bus-web.service
agora-wayfire.service
compositor-bridge.service
```

During agora-de visibility tests, `agora-shell-panel.service` and
`event-bus-web.service` should stay stopped/disabled unless a rollback is being
tested. `agora-wayfire.service` and `compositor-bridge.service` remain the
installed compositor/session backend.

## Restart

Restart only the visible agora-de panel:

```bash
systemctl --user restart agora-de-shell-panel.service
```

Restart the full visible agora-de shell stack:

```bash
systemctl --user restart agora-de-shellui.service
systemctl --user restart agora-de-shell-background.service
systemctl --user restart agora-de-shell-panel.service
```

Verify:

```bash
systemctl --user status agora-de-shellui.service agora-de-shell-background.service agora-de-shell-panel.service --no-pager
ss -ltnp 'sport = :17780 or sport = :7780'
```

## Stale Surface Cleanup

If a restart leaves stale agora-de shell surfaces, close them before restarting
the shell again. Layer-shell surfaces cannot be closed through
`compositorctl surface close`, so terminate their client pids or stop the owning
user services:

```bash
systemctl --user stop agora-de-shell-background.service agora-de-shell-panel.service

compositorctl list-surfaces | jq -r '.surfaces[]
  | select((.surface.app_id // "") | startswith("io.agorade."))
  | .client.pid // empty' \
  | xargs -r kill

systemctl --user start agora-de-shell-background.service
systemctl --user restart agora-de-shell-panel.service
```

Then verify the mapped background and panel:

```bash
compositorctl list-surfaces | jq '.surfaces[]
  | select((.surface.app_id // "") | startswith("io.agorade.Shell"))
  | {
      id: .surface.id,
      app_id: .surface.app_id,
      role: .surface.role,
      visible: .visible,
      output_id: (.surface.output_id // .output_id),
      geometry: (.surface.geometry // .geometry),
      frame_count: .frame_count
    }'
```

`visible: true` is not enough for a visual claim. `frame_count: 0` means the
surface is mapped but has not produced a frame-presented signal.

## Old Stack Shutdown

Stop predecessor visible shell services:

```bash
sudo systemctl stop agora-shell-panel.service event-bus-web.service
sudo systemctl disable agora-shell-panel.service event-bus-web.service
```

Rollback:

```bash
sudo systemctl enable --now event-bus-web.service
sudo systemctl enable --now agora-shell-panel.service
```

`agora-wayfire.service` should not be restarted casually: it owns the physical
Wayfire session on VT1. Restart it only as a deliberate compositor-session
recovery step, and expect all Wayland clients to be remapped afterward.

## Live Evidence

Route and surface readback check:

```bash
./harness/live/check-den-k8.py \
  --systemd-units '' \
  --sockets '' \
  --shell-url 'http://127.0.0.1:17780/shell/dist/desktop/?surface=dock' \
  --catalog-url http://127.0.0.1:17780/api/catalog/apps \
  --surfaces-url http://127.0.0.1:17780/api/surfaces \
  --work-controls-url http://127.0.0.1:17780/api/work-surface-controls \
  --surface-app-id io.agorade.ShellBackground \
  --surface-role background
```

Panel readback check:

```bash
./harness/live/check-den-k8.py \
  --systemd-units '' \
  --sockets '' \
  --shell-url 'http://127.0.0.1:17780/shell/dist/desktop/?surface=dock' \
  --surface-app-id io.agorade.ShellPanel \
  --surface-role panel
```

Require frame-presented evidence:

```bash
AGORA_DE_LIVE_REQUIRE_FRAME=1 \
./harness/live/check-den-k8.py \
  --systemd-units '' \
  --sockets '' \
  --shell-url 'http://127.0.0.1:17780/shell/dist/desktop/?surface=dock' \
  --surface-app-id io.agorade.ShellPanel \
  --surface-role panel
```

On the current bridge, per-layer-shell capture is denied and physical output
capture is not exposed as a logical output. A black physical monitor with a
mapped shell panel should remain classified as insufficient or failing evidence,
not as a closed visual claim.

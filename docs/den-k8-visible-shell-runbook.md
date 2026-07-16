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

The visible chrome default is GTK4/WebKit 6 + gtk4-layer-shell. The service
units set `GDK_BACKEND=wayland` and `LD_PRELOAD=/usr/lib/libgtk4-layer-shell.so`
because the current den-k8 GTK4 layer-shell library needs that preload to report
support.

`?surface=background-fallback` remains available for recovery when the panel
service is deliberately disabled, but the normal installed shape is split
background plus panel surfaces.

## Restart

Restart the visible agora-de background:

```bash
systemctl --user restart agora-de-shell-background.service
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
`agora-de-compositorctl surface close`, so terminate their client pids or stop the owning
user services:

```bash
systemctl --user stop agora-de-shell-background.service agora-de-shell-panel.service

agora-de-compositorctl list-surfaces | jq -r '.surfaces[]
  | select((.surface.app_id // "") | startswith("io.agorade."))
  | .client.pid // empty' \
  | xargs -r kill

systemctl --user start agora-de-shell-background.service agora-de-shell-panel.service
```

Then verify the mapped background and panel:

```bash
agora-de-compositorctl list-surfaces | jq '.surfaces[]
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
recovery step, and expect all Wayland clients to be remapped afterward. On
den-k8, the stable unit shape is an active `oneshot` marker that launches
Wayfire through `openvt -c 1 -f -s -- runuser -u agent -- sg seat`; `openvt -w`
can report status 8 after failing to deallocate VT1, and `openvt -e` did not
start Wayfire reliably on this host.

Native toplevel window visibility is controlled by
`deploy/compositor/agora-de-wayfire-window-visibility-config`. The current
installed state enables Wayfire's `decoration` plugin and sets
`preferred_decoration_mode = server`, giving borderless native apps a visible
title bar/border while leaving client-decorated apps and layer-shell chrome
alone. See `docs/den-k8-window-visibility.md` for rollback and evidence.

If the live host gets into a stale compositor/session state, install the
recovery helper once and then use the sudoers-backed kill-all command:

```bash
sudo deploy/shellui/install-den-k8-recovery-tools
sudo /usr/local/sbin/agora-de-kill-all --help
sudo /usr/local/sbin/agora-de-kill-all
```

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
  --workspaces-url http://127.0.0.1:17780/api/workspaces \
  --operator-status-url http://127.0.0.1:17780/api/operator/status \
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

The panel check targets the GTK4 installed panel path. The old GTK3/WebKit2
panel path reserved a bottom exclusive zone but did not visibly paint, leaving a
black bar; keep that path retired unless deliberately reproducing the old bug.

Expected visible dock state:

- `agora-de` brand block.
- `Apps` and `Refresh` controls.
- `Status` launches the read-only shell status utility.
- App entries loaded from `/api/catalog/apps`.
- Running surface entries loaded from `/api/surfaces`.
- `workspace 1`, mapped-surface status, and clock controls.

The first workspace model is intentionally one workspace:

```text
http://127.0.0.1:17780/api/workspaces
```

`workspace 1` is a visible control. Activating it calls
`POST /api/workspaces/action` with `workspaceId: workspace-1` and confirms the
current workspace state; multi-workspace placement is a later compositor/layout
slice.

Open the operator/status utility from the dock with `Status`, or directly with:

```text
http://127.0.0.1:17780/shell/dist/desktop/?surface=operator
```

Its backing route is:

```text
http://127.0.0.1:17780/api/operator/status
```

The view is read-only. It shows service, socket, output, and surface health plus
the copy/paste-safe recovery commands, including
`sudo /usr/local/sbin/agora-de-kill-all`; it does not execute privileged
recovery actions from the shell UI.

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

Per-layer-shell capture is denied; use physical output capture for visible
claims. The active den-k8 monitor is currently exposed as `HDMI-A-1` by
`agora-de-compositorctl output list`.

Run the full installed-service visual check:

```bash
./harness/live/check-den-k8.py \
  --systemd-units 'agora-wayfire.service,compositor-bridge.service' \
  --sockets '/run/agent-os/compositor-control.sock,/run/agent-os/compositor-bridge.sock' \
  --shell-url 'http://127.0.0.1:17780/shell/dist/desktop/?surface=dock' \
  --catalog-url 'http://127.0.0.1:17780/api/catalog/apps' \
  --surfaces-url 'http://127.0.0.1:17780/api/surfaces' \
  --work-controls-url 'http://127.0.0.1:17780/api/work-surface-controls' \
  --workspaces-url 'http://127.0.0.1:17780/api/workspaces' \
  --operator-status-url 'http://127.0.0.1:17780/api/operator/status' \
  --surface-app-id io.agorade.ShellPanel \
  --surface-role panel \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-live \
  --require-capture
```

The capture packet should report `captureClassification: capture_visible` and
`pixelClassification.classification: expected_shell_visible`. If the monitor is
black or the output capture path fails, the harness fails closed and includes the
artifact path or agora-de-compositorctl stderr needed for debugging.

For task 4153, the live artifact
`/run/agent-os/artifacts/den-k8-live-4153/output-capture-1783170766443939863-3/output-capture-1783170766443939863-3.png`
showed the API-backed dock controls and passed the installed-service capture
check.

## Launch Loop

Check the first app launch/focus/close loop through shellui:

```bash
./harness/live/check-shell-loop.py \
  --base-url http://127.0.0.1:17780
```

The loop verifies:

- `example-browser` is present and launchable in `/api/catalog/apps`.
- `POST /api/catalog/launch` returns a tracked launch and surface id.
- `/api/surfaces` reports the launched toplevel surface as running.
- `POST /api/surfaces/action` accepts `focus` and `close`.
- The closed surface disappears from running state.

Close visible launch-loop claims with physical output capture:

```bash
./harness/live/check-shell-loop.py \
  --base-url http://127.0.0.1:17780 \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-shell-loop \
  --require-capture
```

The capture-backed loop emits a `den-k8-shell-launch-visible` evidence packet
after the app is running and focused, before close/stale-cleanup. The packet
should report `captureClassification: capture_visible` and
`pixelClassification.classification: expected_shell_visible`. The runner waits
briefly before capture so the dock can refresh from `/api/surfaces` and show the
running app entry.

For task 4154, the launch loop passed 6/6 checks and the physical output
artifact
`/run/agent-os/artifacts/den-k8-live-4154/output-capture-1783172629534818648-4/output-capture-1783172629534818648-4.png`
showed the launched app window plus dock running-state controls.

## GTK3 vs GTK4 Bake-Off

Run the live GTK layer-shell comparison:

```bash
./harness/live/gtk-layer-shell-bakeoff.py \
  --output /tmp/agora-de-gtk-layer-shell-bakeoff.json
```

Hold one case on the monitor for physical observation:

```bash
./harness/live/gtk-layer-shell-bakeoff.py \
  --cases gtk4-panel \
  --hold-seconds 45 \
  --output /tmp/agora-de-gtk4-panel-hold.json
```

The current host needs `LD_PRELOAD=/usr/lib/libgtk4-layer-shell.so` for
GTK4/Gtk4LayerShell support; the runner applies that automatically for GTK4
cases.

Human observation on 2026-07-04 confirmed the GTK4 panel bake-off visibly
painted on the physical monitor. The installed agora-de panel service now uses
the same GTK4/WebKit 6 + gtk4-layer-shell family by default.

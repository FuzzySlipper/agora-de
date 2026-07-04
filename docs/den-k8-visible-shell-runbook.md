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
`compositorctl surface close`, so terminate their client pids or stop the owning
user services:

```bash
systemctl --user stop agora-de-shell-background.service agora-de-shell-panel.service

compositorctl list-surfaces | jq -r '.surfaces[]
  | select((.surface.app_id // "") | startswith("io.agorade."))
  | .client.pid // empty' \
  | xargs -r kill

systemctl --user start agora-de-shell-background.service agora-de-shell-panel.service
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
recovery step, and expect all Wayland clients to be remapped afterward. On
den-k8, the stable unit shape is an active `oneshot` marker that launches
Wayfire through `openvt -c 1 -f -s -- runuser -u agent -- sg seat`; `openvt -w`
can report status 8 after failing to deallocate VT1, and `openvt -e` did not
start Wayfire reliably on this host.

If the live host gets into a stale compositor/session state, install the
recovery helper once and then use the sudoers-backed kill-all command:

```bash
sudo deploy/shellui/install-den-k8-recovery-tools
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

The long-term evidence target is physical output capture for the active monitor
such as `HDMI-A-1`. Once `compositorctl output list` exposes that output and
capture returns image evidence, output capture should outrank mapped state and
frame counters for visible-shell closeout.

During the transition, `compositorctl output list` may expose
`mode: physical_surface_readback`. That mode means the bridge inferred the
physical output identity from mapped surface readback. It is useful for choosing
the output name, but it is not yet pixel evidence.

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

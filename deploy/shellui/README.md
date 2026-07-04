# Shellui Deployment Testing

This directory contains productization material for running the first
deployment-testing slice of agora-de shellui on den-k8.

It does not contain product source. Build the binary from `go/cmd/shellui` and
install it into an operator-chosen artifact path.

## Build

From the repo root:

```bash
go build -C go -o /opt/agora-de/bin/shellui ./cmd/shellui
```

The service can run with fixture providers while deployment plumbing is being
tested:

```bash
/opt/agora-de/bin/shellui --listen 127.0.0.1:7780 --fixture-providers=true
```

## User Systemd

For den-k8 deployment testing, prefer a user service. This mirrors the host's
current user-service style and does not require sudo.

Suggested install paths:

```text
~/.config/systemd/user/agora-de-shellui.service
~/.config/systemd/user/agora-de-shell-background.service
~/.config/systemd/user/agora-de-shell-panel.service
~/.config/agora-de/shellui.env
~/.local/bin/agora-de-shellui
~/.local/bin/agora-de-gtk4-layer-shell-webview
~/.local/bin/agora-de-shell-panel-supervisor
~/.local/share/agora-de/shell/dist
```

Install and start:

```bash
go build -C go -o ~/.local/bin/agora-de-shellui ./cmd/shellui
install -D -m 0644 deploy/shellui/agora-de-shellui.user.service ~/.config/systemd/user/agora-de-shellui.service
install -D -m 0644 deploy/shellui/agora-de-shell-background.user.service ~/.config/systemd/user/agora-de-shell-background.service
install -D -m 0644 deploy/shellui/agora-de-shell-panel.user.service ~/.config/systemd/user/agora-de-shell-panel.service
install -D -m 0755 deploy/shellui/agora-de-gtk4-layer-shell-webview ~/.local/bin/agora-de-gtk4-layer-shell-webview
install -D -m 0755 deploy/shellui/agora-de-shell-panel-supervisor ~/.local/bin/agora-de-shell-panel-supervisor
install -D -m 0644 deploy/shellui/shellui.user.env.example ~/.config/agora-de/shellui.env
systemctl --user daemon-reload
systemctl --user enable --now agora-de-shellui.service
systemctl --user enable --now agora-de-shell-background.service
systemctl --user enable --now agora-de-shell-panel.service
```

The user-service example uses `127.0.0.1:17780` to avoid colliding with the
currently bound predecessor port `7780`. Move it to `7780` once that port is
free or intentionally replaced.

`agora-de-shell-background.service` launches the background shell and
`agora-de-shell-panel.service` launches the dock/panel shell. Both use the
repo-owned GTK4/WebKit 6 + gtk4-layer-shell helper by default. On the current
den-k8 host GTK4 layer-shell support requires:

```text
GDK_BACKEND=wayland
LD_PRELOAD=/usr/lib/libgtk4-layer-shell.so
```

Those values are present in the user-service examples. The
`?surface=background-fallback` route keeps the temporary background-owned
taskbar available as a recovery path; it is not the default installed shape.

## System Systemd

Suggested install paths:

```text
/etc/systemd/system/agora-de-shellui.service
/etc/agora-de/shellui.env
/opt/agora-de/bin/shellui
/opt/agora-de/shell/dist
```

Copy:

```bash
install -D -m 0644 deploy/shellui/agora-de-shellui.service /etc/systemd/system/agora-de-shellui.service
install -D -m 0644 deploy/shellui/shellui.env.example /etc/agora-de/shellui.env
systemctl daemon-reload
systemctl enable --now agora-de-shellui.service
```

System service installation requires root. The predecessor
`agora-shell-panel.service` in `../agora-os` was a system unit with `User=agent`,
not a true user unit.

The current service intentionally uses fixture providers by default for
deployment testing. Disable `AGORA_DE_SHELLUI_FIXTURE_PROVIDERS` only after a
live compositor/provider path is wired.

To use the installed compositor bridge through `compositorctl list-surfaces`,
set:

```text
AGORA_DE_SHELLUI_SURFACE_PROVIDER=compositorctl
AGORA_DE_SHELLUI_COMPOSITORCTL=/usr/local/bin/compositorctl
```

If that command fails, `/api/surfaces` and `/api/work-surface-controls` fail
closed with HTTP 503. Fixture data is not used as a fallback in compositorctl
mode.

## Live Evidence

Run the installed-service route evidence:

```bash
AGORA_DE_LIVE_SHELL_URL=http://127.0.0.1:7780/shell/dist/desktop/?surface=dock \
AGORA_DE_LIVE_CATALOG_URL=http://127.0.0.1:7780/api/catalog/apps \
AGORA_DE_LIVE_SURFACES_URL=http://127.0.0.1:7780/api/surfaces \
AGORA_DE_LIVE_WORK_CONTROLS_URL=http://127.0.0.1:7780/api/work-surface-controls \
./harness/live/check-den-k8.py
```

For route-only testing without systemd/socket checks:

```bash
./harness/live/check-den-k8.py \
  --systemd-units '' \
  --sockets '' \
  --shell-url http://127.0.0.1:7780/shell/dist/desktop/?surface=dock \
  --catalog-url http://127.0.0.1:7780/api/catalog/apps \
  --surfaces-url http://127.0.0.1:7780/api/surfaces \
  --work-controls-url http://127.0.0.1:7780/api/work-surface-controls
```

These route checks prove installed model/route shape. User-visible visual claims
still require capture/readback evidence.

For the den-k8 user-service restart and visibility playbook, see
`docs/den-k8-visible-shell-runbook.md`.

## Recovery Helper

The live den-k8 compositor path can accumulate stale Wayland, WebKit, or bridge
state while this successor repo is still replacing the predecessor shell. Install
the root-owned recovery helper once:

```bash
sudo deploy/shellui/install-den-k8-recovery-tools
```

That installs `/usr/local/sbin/agora-de-kill-all` and a sudoers entry allowing
the `agent` user to run it without a password:

```bash
sudo /usr/local/sbin/agora-de-kill-all
```

The helper intentionally leaves Agora display/session services stopped. Start or
deploy them again only after the host is back to an empty state.

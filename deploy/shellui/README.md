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
~/.config/systemd/user/agora-de-shell-panel.service
~/.config/agora-de/shellui.env
~/.local/bin/agora-de-shellui
~/.local/bin/agora-de-shell-panel-supervisor
~/.local/share/agora-de/shell/dist
```

Install and start:

```bash
go build -C go -o ~/.local/bin/agora-de-shellui ./cmd/shellui
install -D -m 0644 deploy/shellui/agora-de-shellui.user.service ~/.config/systemd/user/agora-de-shellui.service
install -D -m 0644 deploy/shellui/agora-de-shell-panel.user.service ~/.config/systemd/user/agora-de-shell-panel.service
install -D -m 0755 deploy/shellui/agora-de-shell-panel-supervisor ~/.local/bin/agora-de-shell-panel-supervisor
install -D -m 0644 deploy/shellui/shellui.user.env.example ~/.config/agora-de/shellui.env
systemctl --user daemon-reload
systemctl --user enable --now agora-de-shellui.service
systemctl --user enable --now agora-de-shell-panel.service
```

The user-service example uses `127.0.0.1:17780` to avoid colliding with the
currently bound predecessor port `7780`. Move it to `7780` once that port is
free or intentionally replaced.

`agora-de-shell-panel.service` launches a layer-shell panel webview pointed at
the user shellui service through `compositorctl`, then keeps systemd attached to
the mapped surface and WebView client process. It is intentionally separate from
the HTTP service so route evidence and visible-monitor evidence can be restarted
independently.

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

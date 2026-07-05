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

The default dock/panel route is API-backed. `?surface=dock` renders the Agora DE
brand, `Apps`, `Refresh`, and `Status` controls, app entries from
`/api/catalog/apps`, running surface entries from `/api/surfaces`, workspace
state, mapped-surface status, and a clock.

The first workspace model is intentionally conservative. `/api/workspaces`
reports one active `workspace-1`, and `POST /api/workspaces/action` with
`{"workspaceId":"workspace-1","action":"activate"}` confirms that workspace
state without pretending multi-workspace compositor placement exists yet.

Catalog provider settings:

```text
AGORA_DE_SHELLUI_CATALOG_PROVIDER=fixture
AGORA_DE_SHELLUI_DESKTOP_ENTRY_ROOTS=/usr/share/applications:/home/agent/.local/share/applications
AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER=disabled
AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST=
```

`fixture` is still the live default because it includes the explicit
`example-browser` and `shell-status` launch targets used by the installed
evidence loops. `desktop_entries` imports installed `.desktop` entries from the
configured roots and exposes them through `/api/catalog/apps`, but entries
without an explicit shellui launch target render as non-launchable until the
native launch boundary exists. See `docs/native-launch-policy.md` for the
current launchability contract.

The first launch loop is also shellui-backed: `POST /api/catalog/launch` maps a
known catalog app to a compositor launch target, and `POST /api/surfaces/action`
accepts `focus` and `close` for running work surfaces. Use
`harness/live/check-shell-loop.py` to verify launch, running-state readback,
focus, close, and stale-entry cleanup against the installed service.

The `Status` control launches `?surface=operator`, a read-only shell status
utility backed by `/api/operator/status`. It exposes service/socket/output and
surface health plus copy/paste-safe recovery commands. Privileged recovery stays
behind the installed `/usr/local/sbin/agora-de-kill-all` helper and sudoers
boundary; the web UI does not run it directly.

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
AGORA_DE_LIVE_WORKSPACES_URL=http://127.0.0.1:7780/api/workspaces \
AGORA_DE_LIVE_OPERATOR_STATUS_URL=http://127.0.0.1:7780/api/operator/status \
AGORA_DE_LIVE_SURFACE_APP_ID=io.agorade.ShellPanel \
AGORA_DE_LIVE_SURFACE_ROLE=panel \
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
  --work-controls-url http://127.0.0.1:7780/api/work-surface-controls \
  --workspaces-url http://127.0.0.1:7780/api/workspaces \
  --operator-status-url http://127.0.0.1:7780/api/operator/status \
  --surface-app-id io.agorade.ShellPanel \
  --surface-role panel
```

The route checks prove installed model/route shape. The compositor surface
readback adds mapped/visible and content-commit evidence. User-visible visual
claims close with physical output capture and expected shell-pixel
classification. The launch-loop harness can run the app lifecycle and capture
the monitor in one pass with `--output-name HDMI-A-1 --require-capture`.

To validate the desktop-entry catalog provider without replacing the visible
fixture launch service, run a sidecar shellui on a temporary port:

```bash
cd /home/dev/agora-de/go
go run ./cmd/shellui \
  --listen 127.0.0.1:17782 \
  --catalog-provider desktop_entries \
  --desktop-entry-roots /usr/share/applications:/home/agent/.local/share/applications \
  --surface-provider fixture
```

Then check the imported catalog route from another shell:

```bash
./harness/live/check-installed-catalog.py \
  --catalog-url http://127.0.0.1:17782/api/catalog/apps \
  --min-apps 1 \
  --require-all-nonlaunchable
```

The `--require-all-nonlaunchable` assertion matches
`docs/native-launch-policy.md`: installed entries are discoverable, but native
launch remains deferred until a governed launcher boundary exists.

Native desktop-entry launch has an explicit off switch and remains disabled in
the installed examples:

```text
AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER=disabled
```

The implementation path for later live evidence is
`AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER=structured_compositorctl` plus an
allowlist of desktop-entry ids. That mode expects compositorctl to support the
structured `--arg` launch contract from `docs/native-launch-boundary-design.md`;
do not enable it against a bridge that only supports `-cmd` strings.

For den-k8 native-launch sidecar testing, build the agora-de structured
launcher separately and point only that sidecar at it:

```bash
cd /home/dev/agora-de/go
go build -trimpath -o /home/agent/.local/bin/agora-de-compositorctl ./cmd/compositorctl
```

Keep the default installed shell service on `/usr/local/bin/compositorctl` until
the remaining surface readback, close, and capture operations are lifted.

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

# Shellui Deployment Testing

This directory contains productization material for running the first
deployment-testing slice of agora-de shellui on an existing Linux install.

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

For existing-Linux deployment testing, prefer user services for shell chrome.
This mirrors the current tested host's style and does not require sudo for the
shell surfaces.

Suggested install paths:

```text
~/.config/systemd/user/agora-de-shellui.service
~/.config/systemd/user/agora-de-shell-background.service
~/.config/systemd/user/agora-de-shell-panel.service
~/.config/systemd/user/agora-de-shell-overlay.service
~/.config/agora-de/shellui.env
~/.local/bin/agora-de-shellui
~/.local/bin/agora-de-gtk4-layer-shell-webview
~/.local/bin/agora-de-native-overlay
~/.local/bin/agora-de-shell-panel-supervisor
~/.local/share/agora-de/shell/dist
```

Install and start:

```bash
deploy/shellui/install-user-services --enable --restart
```

Use `--enable-overlay` when the diagnostic overlay should be installed and
started too:

```bash
deploy/shellui/install-user-services --enable --enable-overlay --restart
```

The installer builds `agora-de-shellui`, installs the shell service units and
chrome helpers, and creates `~/.config/agora-de/shellui.env` with current-user
defaults only when the file does not already exist. Use `--overwrite-env` only
when intentionally replacing local settings.

The user-service example uses `127.0.0.1:17780`. This avoids colliding with
other local services while keeping shell traffic local to the user session.

`agora-de-shell-background.service` launches the background shell and
`agora-de-shell-panel.service` launches the dock/panel shell through the
repo-owned GTK4/WebKit 6 + gtk4-layer-shell helper.
`agora-de-shell-overlay.service` launches the agent-visible label/bounds
overlay through the repo-owned GTK4/Cairo native overlay helper, so diagnostics
do not depend on WebKit transparent-background behavior. On the current den-k8
host GTK4 layer-shell support requires:

```text
GDK_BACKEND=wayland
LD_PRELOAD=/usr/lib/libgtk4-layer-shell.so
```

The default dock/panel route is API-backed. `?surface=dock` renders the Agora DE
brand, `Apps`, `Refresh`, and `Status` controls, app entries from
`/api/catalog/apps`, running surface entries from `/api/surfaces`, workspace
state, mapped-surface status, and a clock.

The overlay service still uses `?surface=overlay` as its route identity, but the
installed diagnostics surface is drawn by `agora-de-native-overlay`. The helper
polls `/api/layout` and `/api/surfaces`, renders stable numbered labels,
app/title badges, focus indication, zone hints, and geometry bounds, and clears
the rest of the layer to transparent so native work-surface pixels remain
visible in physical captures.

The overlay service is intentionally not enabled by the default user-service
install path. It is an opt-in diagnostic surface:

```bash
systemctl --user enable --now agora-de-shell-overlay.service
systemctl --user disable --now agora-de-shell-overlay.service
```

The first workspace model is intentionally conservative. `/api/workspaces`
reports one active `workspace-1`, and `POST /api/workspaces/action` with
`{"workspaceId":"workspace-1","action":"activate"}` confirms that workspace
state without pretending multi-workspace compositor placement exists yet.

Catalog provider settings:

```text
AGORA_DE_SHELLUI_CATALOG_PROVIDER=fixture
AGORA_DE_SHELLUI_DESKTOP_ENTRY_ROOTS=/usr/share/applications:$HOME/.local/share/applications
AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER=disabled
AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST=
```

`fixture` is still the live default because it includes the explicit
`example-browser` and `shell-status` launch targets used by the installed
evidence loops. `desktop_entries` imports installed `.desktop` entries from the
configured roots and exposes them through `/api/catalog/apps`, but entries
without an explicit shellui launch target render as non-launchable unless
`structured_compositorctl` is enabled and the desktop-entry id is allowlisted.
Allowlist matching is by id such as `Alacritty.desktop`, not executable path or
display label. The special allowlist value `*` enables every installed entry
that the structured launcher can safely prepare. See
`docs/native-launch-boundary-design.md` for the current launchability contract
and disabled-code vocabulary.

Minimal governed native launch settings on the current tested host:

```text
AGORA_DE_SHELLUI_CATALOG_PROVIDER=desktop_entries
AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER=structured_compositorctl
AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST=Alacritty.desktop
AGORA_DE_SHELLUI_NATIVE_LAUNCH_UID=1001
AGORA_DE_SHELLUI_NATIVE_LAUNCH_GID=1002
AGORA_DE_SHELLUI_NATIVE_LAUNCH_SESSION_TOKEN=session-native-shell
AGORA_DE_SHELLUI_NATIVE_LAUNCH_OUTPUT=HDMI-A-1
AGORA_DE_SHELLUI_NATIVE_LAUNCH_HOME=/home/agent
AGORA_DE_SHELLUI_COMPOSITORCTL=/home/agent/.local/bin/agora-de-compositorctl
```

For a local development install, use
`AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST=*` or
`~/.local/bin/agora-de-native-launch-config enable-all --restart` to make every
structured-launchable installed entry clickable.

Rollback is setting `AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER=disabled`,
clearing `AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST`, and restarting
`agora-de-shellui.service`.

The helper script keeps that edit copy/paste-safe:

```bash
install -D -m 0755 deploy/shellui/agora-de-native-launch-config ~/.local/bin/agora-de-native-launch-config
~/.local/bin/agora-de-native-launch-config enable-alacritty --restart
~/.local/bin/agora-de-native-launch-config enable-all --restart
~/.local/bin/agora-de-native-launch-config disable --restart
```

`agora-de-native-launch-config` derives UID, GID, home, desktop-entry roots, and
the compositorctl path from the current user by default. Override the matching
`AGORA_DE_SHELLUI_*` environment variables when rehearsing a different user or
output.

The first launch loop is also shellui-backed: `POST /api/catalog/launch` maps a
known catalog app to a compositor launch target, and `POST /api/surfaces/action`
accepts `focus` and `close` for running work surfaces. Use
`harness/live/check-shell-loop.py` to verify launch, running-state readback,
focus, close, and stale-entry cleanup against the installed service.

Use `harness/live/check-native-launch.py` for the governed installed-app path.
It requires the agora-de compositorctl path, verifies an allowlisted catalog app
such as `Alacritty.desktop`, launches it through shellui, captures `HDMI-A-1`,
and closes the native surface.

Use `harness/live/check-overlay-labels.py` for the agent overlay path. It
requires `io.agorade.ShellOverlay` to be mapped, launches at least two native
apps through shellui, focuses each app, captures the physical output after focus
changes, and closes the launched surfaces. The capture check fails unless the
same output image contains overlay annotation pixels and native app pixels.

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

To use the installed compositor control path through agora-de `compositorctl list-surfaces`,
set:

```text
AGORA_DE_SHELLUI_SURFACE_PROVIDER=compositorctl
AGORA_DE_SHELLUI_COMPOSITORCTL=/home/agent/.local/bin/agora-de-compositorctl
```

If that command fails, `/api/surfaces` and `/api/work-surface-controls` fail
closed with HTTP 503. Fixture data is not used as a fallback in compositorctl
mode.

## Live Evidence

Run the installed-service route evidence:

```bash
./harness/live/check-den-k8.py \
  --output-name HDMI-A-1 \
  --require-capture
```

For route-only testing without systemd/socket checks:

```bash
./harness/live/check-den-k8.py \
  --systemd-units '' \
  --sockets '' \
  --shell-url http://127.0.0.1:17780/shell/dist/desktop/?surface=dock \
  --catalog-url http://127.0.0.1:17780/api/catalog/apps \
  --surfaces-url http://127.0.0.1:17780/api/surfaces \
  --work-controls-url http://127.0.0.1:17780/api/work-surface-controls \
  --workspaces-url http://127.0.0.1:17780/api/workspaces \
  --operator-status-url http://127.0.0.1:17780/api/operator/status \
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
cd go
go run ./cmd/shellui \
  --listen 127.0.0.1:17782 \
  --catalog-provider desktop_entries \
  --desktop-entry-roots /usr/share/applications:$HOME/.local/share/applications \
  --surface-provider fixture
```

Then check the imported catalog route from another shell:

```bash
./harness/live/check-installed-catalog.py \
  --catalog-url http://127.0.0.1:17782/api/catalog/apps \
  --min-apps 1 \
  --require-all-nonlaunchable \
  --require-disabled-codes \
  --require-disabled-reasons
```

The `--require-all-nonlaunchable` assertion is for disabled or non-allowlisted
sidecar checks. When `structured_compositorctl` is enabled, the installed
catalog should report only allowlisted desktop-entry ids as launchable.

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

For native-launch and live readback testing, install the agora-de compositor
bridge and compositor control launcher:

```bash
sudo deploy/compositor/install-compositor-bridge-service.sh
```

This repo-owned command now covers the installed shellui readback, focus/close,
physical output capture, and structured native-launch command surface. It still
speaks to the installed Wayfire bridge service socket.

For the current tested host's user-service restart and visibility playbook, see
`docs/den-k8-visible-shell-runbook.md`.

## Recovery Helper

The live compositor path can accumulate stale Wayland, WebKit, or bridge state
while developing. Install the root-owned recovery helper once:

```bash
sudo deploy/shellui/install-recovery-tools
```

That installs `/usr/local/sbin/agora-de-kill-all` and a sudoers entry allowing
the invoking sudo user to run it without a password. Set `AGORA_DE_SUDO_USER`
when installing for a different desktop user:

```bash
sudo /usr/local/sbin/agora-de-kill-all
```

Help and argument validation are side-effect free:

```bash
sudo /usr/local/sbin/agora-de-kill-all --help
```

The helper intentionally leaves Agora display/session services stopped. Start or
deploy them again only after the host is back to an empty state.

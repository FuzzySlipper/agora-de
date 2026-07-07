# Deployed WM Model

This document describes the current deployed agora-de window-manager path. It is
about agora-de as a desktop environment for an existing Linux install; it does
not require agora-os governance services, predecessor shell shims, or old
runtime modes.

## Independence

Agora-de owns desktop environment concerns in this repo:

- compositor bridge and layout state;
- shellui HTTP projection, dock, launcher, status, and overlay surfaces;
- native app launch through structured shell/catalog routes;
- live evidence harnesses for installed-service testing on the current host.

Agora-os remains predecessor evidence and, later, a possible typed governance
service peer. It is not a runtime dependency for the deployed WM path. Product
code must not import `../agora-os`, read governance logs as APIs, or add legacy
fallbacks to keep the old DE alive.

## User Model

The default usable workspace is a deterministic tiled workspace:

- normal app toplevels enter `zones` mode and are placed by the current
  `master_stack` rule;
- the focused tiled surface is promoted into the master/primary area;
- stack surfaces are placed in stable order with backend-acknowledged geometry;
- shell chrome, launcher/status surfaces, dialogs, menus, and explicit floating
  overrides remain transient or floating instead of participating in tiling;
- the installed bottom panel is reserved as shell chrome.

The visible dock exposes the current working controls:

- `Apps` opens or closes the launcher;
- `Status` opens the read-only operator surface;
- running app buttons focus a surface;
- `Prev` and `Next` focus adjacent tiled surfaces by backend-reported order;
- `Master` focuses the current target so auto-layout promotes it;
- `Move` asks the backend to move the target to the next reported zone;
- `Float` / `Tile` toggles backend-owned participation through
  `/api/layout/action`;
- `Full`, `Max`, and `Close` go through `/api/surfaces/action`;
- `Reset` reapplies the current layout mode, or enters `zones` from `freeform`;
- the rule chip shows backend-reported layout rule/settings/revision.

Settings are backend-owned. Today they are persisted by the compositor bridge at
`$XDG_CONFIG_HOME/agora-de/layout-settings.json`, or at the path supplied by
`AGORA_DE_LAYOUT_SETTINGS`. The persisted shape includes mode, rule, gaps,
master count, master ratio, and smart gaps. Shellui displays those settings but
does not compute placement policy.

Known current limitations:

- output-aware workspace state and shell controls are implemented, including
  optional output selectors for workspace activation. Live evidence on den-k8
  currently proves single-output stability only because the host reports one
  physical output; true multi-monitor behavior still needs soak evidence on a
  host with multiple outputs;
- fullscreen, maximize, and minimize are compositor-backed in the installed
  Wayfire adapter through `set_surface_state`, and shell-facing routes accept
  explicit enable/disable state for reversible checks; future adapters must
  either expose equivalent acknowledged state changes or return
  `backend_unsupported`;
- the shell overlay is an evidence layer, not a native app wrapper;
- Wayfire is the current installed backend, but layout policy is not Wayfire
  product policy.

## Agent Model

Agents should treat structured state as authority and screenshots as evidence.

Primary state:

- `GET /api/layout` returns camelCase layout state for shell and agents:
  mode, revision, settings, surfaces, workspaces, zones, stable labels, focus,
  order, participation, and geometry.
- `agora-de-compositorctl layout get` returns the backend JSON contract with
  snake_case fields.
- `GET /api/surfaces` returns running surface identity and lifecycle state.
- `?surface=overlay` renders capture-visible labels, bounds, zone hints,
  focus state, and action hints over native app surfaces.

Primary actions:

- `POST /api/layout/action` for `setMode`, `assignZone`, `tile`,
  `moveResize`, `setFloating`, and `activateWorkspace`.
- `POST /api/surfaces/action` for `focus`, `close`, `maximize`, `minimize`,
  `fullscreen`, and shell-facing surface actions.
- `agora-de-compositorctl surface ...` for agent/operator command evidence.

Agents should target surfaces by stable surface id and layout label. They should
not infer geometry from browser rectangles, screenshot pixels, or webview
wrappers. Evidence harnesses distinguish failure classes such as `launch`,
`visibility`, `planner-mismatch`, `backend-placement`, `occlusion`,
`focus-order`, `shell-action`, `agent-action`, `overlay`, `capture`, `restart`,
and `cleanup`.

## Backend Boundary

The current boundary is:

- Rust layout crates own backend-independent command semantics and planner
  policy.
- The compositor backend applies desired rectangles and reports truthful
  post-layout geometry, focus/order, workspace/zone state, revisions, and
  support/unsupported outcomes.
- Go bridge code transports and caches backend facts, persists settings, and
  exposes the shell-facing route surface.
- TypeScript/shellui projects state and asks for actions; it does not own
  placement policy.

Wayfire remains the installed evidence backend because the current proof can
apply and acknowledge layout geometry without synthetic keyboard shortcuts or
shell inference. Move primary backend work toward Smithay/Rust or another
backend if a required action needs screenshot-derived geometry, synthetic key
bindings, shell-side placement inference, or Wayfire plugin scope grows beyond
the bounded adapter role in `docs/compositor-backend-decision.md`.

## Install And Update

The existing-Linux install uses the compositor bridge as a system service and
shell surfaces as user services. It requires Wayfire plus the GTK4/WebKit and
gtk4-layer-shell stack used by the shell chrome helpers. It does not require
agora-os services.

From the repo root, build and install or update the compositor bridge:

```bash
sudo deploy/compositor/install-compositor-bridge-service.sh
```

Install or update the user shell services and helper binaries:

```bash
deploy/shellui/install-user-services --enable --enable-overlay --restart
```

The installer preserves an existing `~/.config/agora-de/shellui.env`. Pass
`--overwrite-env` only when intentionally replacing local settings. Enable
native launch for all structured-launchable desktop entries on this development
host with:

```bash
~/.local/bin/agora-de-native-launch-config enable-all --restart
```

Install recovery helpers once when sudo-backed cleanup is desired:

```bash
sudo deploy/shellui/install-recovery-tools
sudo deploy/compositor/install-compositor-tools
```

Update after pulling new agora-de code:

```bash
sudo deploy/compositor/install-compositor-bridge-service.sh
deploy/shellui/install-user-services --restart
sudo /usr/local/sbin/agora-de-compositor-bridge-admin restart-bridge
```

Restart only shell surfaces:

```bash
systemctl --user restart agora-de-shellui.service
systemctl --user restart agora-de-shell-background.service agora-de-shell-panel.service agora-de-shell-overlay.service
```

Recover stale launched clients and shell surfaces:

```bash
sudo /usr/local/sbin/agora-de-kill-all --help
sudo /usr/local/sbin/agora-de-kill-all
systemctl --user restart agora-de-shellui.service
systemctl --user restart agora-de-shell-background.service agora-de-shell-panel.service agora-de-shell-overlay.service
```

## Checks And Evidence

Run local CI:

```bash
./harness/ci/check-all.sh
```

Focused checks:

```bash
./harness/ci/check-go.sh
./harness/ci/check-ts.sh
./harness/ci/check-depgraph.sh
./harness/ci/check-compositor.sh
./harness/ci/check-live-harnesses.sh
```

Run deployed WM evidence against the installed service:

```bash
./harness/live/check-auto-tiling-wm.py \
  --base-url http://127.0.0.1:17780 \
  --app-id Alacritty.desktop \
  --app-id foot.desktop \
  --app-id firefox.desktop \
  --expected-app-id Alacritty \
  --expected-app-id foot \
  --expected-app-id firefox \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-auto-tiling-wm \
  --require-capture
```

Useful supporting live harnesses:

```bash
./harness/live/check-den-k8.py \
  --systemd-units compositor-bridge.service,agora-wayfire.service \
  --sockets /run/agent-os/compositor-bridge.sock,/run/agent-os/compositor-control.sock \
  --shell-url http://127.0.0.1:17780/shell/dist/desktop/?surface=dock \
  --catalog-url http://127.0.0.1:17780/api/catalog/apps \
  --surfaces-url http://127.0.0.1:17780/api/surfaces \
  --work-controls-url http://127.0.0.1:17780/api/work-surface-controls \
  --workspaces-url http://127.0.0.1:17780/api/workspaces \
  --operator-status-url http://127.0.0.1:17780/api/operator/status \
  --surface-app-id io.agorade.ShellPanel \
  --surface-role panel \
  --output-name HDMI-A-1 \
  --require-capture
./harness/live/check-native-launch.py --output-name HDMI-A-1 --require-capture
./harness/live/check-overlay-labels.py --output-name HDMI-A-1 --require-capture
./harness/live/check-planner-layout.py --output-name HDMI-A-1 --require-capture
./harness/live/check-multi-output-workspaces.py
```

The live harnesses target the installed service on the host. They do not create
VMs and they do not require agora-os to be running.

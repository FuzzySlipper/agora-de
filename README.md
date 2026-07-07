# agora-de

Successor desktop-environment repository for the Agora GUI/DE concern.

`agora-de` owns desktop environment behavior for an existing Linux install:
compositor mediation, deterministic window layout, shell chrome, native app
launch, theming, and live visual evidence. It is not an `agora-os`
compatibility shim, and it does not require agora-os governance services or the
old shell runtime to run.

The auto-tiling WM northstar is closed as of Den task 4452. The installed
den-k8 service now proves a usable tiled desktop with native apps, taskbar and
workspace controls, structured agent APIs, overlay evidence, recovery helpers,
and CI-backed live harnesses.

## Current State

- Existing-Linux deployment is the primary target. Live validation runs against
  the installed service on the host, not VMs.
- Wayfire is the current evidence backend. Layout policy is intentionally
  backend-neutral and lives behind the compositor boundary.
- Rust owns new value types, contracts, layout semantics, evidence models, and
  future backend experiments.
- Go carries the working bridge, shell HTTP projection, native launch path, and
  installed-service operations while the successor split continues.
- TypeScript is reserved for shell/customizer projection and generated protocol
  consumption.
- `../agora-os` is predecessor evidence only. Product code must not import its
  internals or add old-runtime fallbacks.

## What Works

- Deterministic auto-tiling with `zones`, `columns`, and backend-owned
  workspaces.
- Master/stack layout planning through Rust fixtures and compositor bridge
  acknowledgement.
- Native launch from desktop entries through structured shell/catalog routes.
- A bottom shell panel with launcher, status, running tasks, workspace controls,
  focus/promote/move/float/tile/fullscreen/maximize/minimize/close/reset
  controls, and centralized theme tokens.
- Agent-facing state and actions through `/api/layout`, `/api/surfaces`,
  `/api/workspaces`, `/api/catalog/apps`, `/api/catalog/launch`, and
  `agora-de-compositorctl`.
- Route/action timing diagnostics through `/api/diagnostics/timing` and
  `/api/operator/status`.
- Capture-visible native overlay labels for surface ids, bounds, focus, and
  zone hints.
- Installed recovery helpers for compositor bridge restart and live-session
  cleanup.
- GitHub `Verify Agora DE` CI running `./harness/ci/check-all.sh`.

## Repo Layout

```text
docs/          successor briefs, deployed model, runbooks, evidence, decisions
governance/    ownership and dependency rules checked by CI
harness/       CI checks, depgraph checks, fixtures, and live evidence harnesses
de-rs/         Rust authority, protocol, state, evidence, and backend crates
go/            compositor bridge, shellui, native launch, and CLI operations
ts/            TypeScript protocol/shell workspace
compositor/    Wayfire fixtures, backend decisions, and compositor spikes
chrome/        native chrome, overlay, and webview layer-shell helpers
deploy/        install/update/recovery scripts and systemd units
```

Product source belongs in `de-rs/`, `go/`, `ts/`, `compositor/`, or `chrome/`.
`deploy/` is for productization artifacts only.

## First Commands

Install TypeScript dependencies once:

```bash
cd ts
npm install
cd ..
```

Run the full local check suite:

```bash
./harness/ci/check-all.sh
```

Focused checks:

```bash
./harness/ci/check-rust.sh
./harness/ci/check-go.sh
./harness/ci/check-ts.sh
./harness/ci/check-depgraph.sh
./harness/ci/check-contracts.sh
./harness/ci/check-compositor.sh
./harness/ci/check-live-harnesses.sh
```

## Installed Service

The deployed model uses a system compositor bridge plus user shell services.
Install or update the compositor bridge:

```bash
sudo deploy/compositor/install-compositor-bridge-service.sh
```

Install or update shell user services and helper binaries:

```bash
deploy/shellui/install-user-services --enable --enable-overlay --restart
```

Enable native launch for all structured-launchable desktop entries on this
development host:

```bash
~/.local/bin/agora-de-native-launch-config enable-all --restart
```

Update after pulling new code:

```bash
sudo deploy/compositor/install-compositor-bridge-service.sh
deploy/shellui/install-user-services --restart
sudo /usr/local/sbin/agora-de-compositor-bridge-admin restart-bridge
```

Restart shell surfaces only:

```bash
systemctl --user restart agora-de-shellui.service
systemctl --user restart agora-de-shell-background.service agora-de-shell-panel.service agora-de-shell-overlay.service
```

## Live Evidence

The live harnesses target the installed host service. They do not start VMs and
they do not require agora-os to be running.

Core WM closeout harness:

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

Supporting harnesses:

```bash
./harness/live/check-native-launch.py --output-name HDMI-A-1 --require-capture
./harness/live/check-layout-commands.py --output-name HDMI-A-1 --require-capture
./harness/live/check-planner-layout.py --output-name HDMI-A-1 --require-capture
./harness/live/check-daily-wm-workflow.py --output-name HDMI-A-1 --require-capture
./harness/live/check-overlay-labels.py --output-name HDMI-A-1 --require-capture
./harness/live/check-popup-stability.py --output-name HDMI-A-1 --require-capture
./harness/live/check-responsiveness-baseline.py
```

Default installed-service proof:

```bash
./harness/live/check-den-k8.py \
  --shell-url 'http://127.0.0.1:17780/shell/dist/desktop/?surface=dock' \
  --systemd-units compositor-bridge.service \
  --sockets /run/agent-os/compositor-bridge.sock,/run/agent-os/compositor-control.sock \
  --catalog-url http://127.0.0.1:17780/api/catalog/apps \
  --surfaces-url http://127.0.0.1:17780/api/surfaces \
  --work-controls-url http://127.0.0.1:17780/api/work-surface-controls \
  --workspaces-url http://127.0.0.1:17780/api/workspaces \
  --operator-status-url http://127.0.0.1:17780/api/operator/status \
  --surface-app-id io.agorade.ShellPanel \
  --surface-role panel \
  --output-name HDMI-A-1 \
  --require-capture
```

## Recovery

Install recovery helpers once when sudo-backed cleanup is desired:

```bash
sudo deploy/shellui/install-recovery-tools
sudo deploy/compositor/install-compositor-tools
```

Useful recovery commands:

```bash
sudo /usr/local/sbin/agora-de-compositor-bridge-admin status
sudo /usr/local/sbin/agora-de-compositor-bridge-admin restart-bridge
sudo /usr/local/sbin/agora-de-kill-all
systemctl --user restart agora-de-shellui.service
systemctl --user restart agora-de-shell-background.service agora-de-shell-panel.service agora-de-shell-overlay.service
```

## Near-Term Work

The foundations are in place. The next work should optimize the usable desktop
rather than prove the basic architecture again:

- taskbar polish and minimized-window restore affordances;
- richer theme development on the centralized token/config path;
- multi-output workspace policy;
- deeper transient/dialog policy;
- longer-running live-session soak evidence;
- Smithay/Rust backend evaluation behind the same layout/compositor boundary.

## Core References

- [Deployed WM model](docs/deployed-wm-model.md)
- [Auto-tiling WM northstar review](docs/auto-tiling-wm-northstar-review.md)
- [Successor brief](docs/successor-brief.md)
- [Successor lesson packet](docs/successor-lesson-packet.md)
- [Architecture](governance/architecture.md)
- [Ownership](governance/ownership.toml)
- [Agora OS boundary](docs/agora-os-boundary.md)
- [Layout model authority](docs/layout-model-authority.md)
- [Backend-agnostic layout planner](docs/backend-agnostic-layout-planner.md)
- [Compositor backend decision](docs/compositor-backend-decision.md)
- [Structured window handling](docs/structured-window-handling.md)
- [Theme boundary](docs/theme-boundary.md)
- [Native launch policy](docs/native-launch-policy.md)
- [Performance and responsiveness](docs/performance-responsiveness.md)
- [den-k8 visible shell runbook](docs/den-k8-visible-shell-runbook.md)
- [GitHub check gates](docs/github-check-gates.md)

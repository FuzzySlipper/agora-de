# den-k8 Live Evidence

agora-de live evidence targets the installed service on `den-k8` class hosts.
On the current host, `hostname` reports `den-k8plus`; this is treated as the
installed-service environment for agora-de checks.

agora-de does not create, boot, orchestrate, or test against VMs. VM harnesses
belong in agora-os when fundamental OS/runtime behavior needs that level of
coverage. This repo consumes the installed service behavior exposed on den-k8.

## Gate Contract

The live evidence gate is an opt-in installed-service gate. It is not part of
deterministic local CI.

Local CI remains:

```bash
./harness/ci/check-all.sh
```

The live installed-service gate is:

```bash
./harness/live/check-den-k8.py
```

The live runner emits JSON with:

- `checks`: service, shell UI, compositor socket, and optional capture checks;
- `evidencePackets`: records using the generated `EvidencePacket` vocabulary;
- `summary`: pass/fail counts.

## Installed Surfaces

Current den-k8 installed-service surfaces observed on this host:

| Category | Surface | Evidence boundary |
| --- | --- | --- |
| Service health | `event-bus.service` | `systemctl is-active event-bus.service` |
| Service health | `event-bus-web.service` | `systemctl is-active event-bus-web.service` |
| Service health | `compositor-bridge.service` | `systemctl is-active compositor-bridge.service` |
| Service health | `agora-wayfire.service` | `systemctl is-active agora-wayfire.service` |
| Service health | `agora-shell-panel.service` | `systemctl is-active agora-shell-panel.service` |
| Shell UI | `http://127.0.0.1:7780/shell/dist/desktop/?surface=dock` | HTTP 200 shell HTML |
| Shell UI claim | optional app catalog route | JSON payload with `apps` array |
| Shell UI claim | optional surface lifecycle route | JSON payload with `surfaces` array |
| Shell UI claim | optional work surface controls route | JSON payload with `surfaces` array |
| Shell UI claim | optional workspace route | JSON payload with current workspace and workspace array |
| Shell UI claim | optional operator status route | JSON payload with service/socket/output status and recovery commands |
| Compositor/event bus | `/run/agent-os/bus.sock` | Unix socket exists and accepts connection |
| Compositor bridge | `/run/agent-os/compositor-bridge.sock` | Unix socket exists and accepts connection |
| Compositor control | `/run/agent-os/compositor-control.sock` | Unix socket exists and accepts connection |
| Capture artifacts | `/run/agent-os/captures` | capture JSON supplied to the runner |
| Surface frame readback | `compositorctl list-surfaces` | selected surface mapped/visible and optional `frame_count` |
| Physical output discovery | `compositorctl output list` | physical output identity; `physical_surface_readback` is inferred from mapped surfaces |
| Physical output capture | `compositorctl output capture` via `--output-name` | monitor/output image evidence, preferred for visible-shell closeout |

The shell route currently proves installed shell availability, not visual
correctness. `/api/surfaces` forwards compositor lifecycle fields such as
`contentCommitCount` when the compositor provider is enabled. Visual claims
close through physical output capture when available; human confirmation is now
fallback evidence, not the preferred closeout path.

The current GTK4 dock/panel visible state is intentionally small but usable:
brand, `Apps`, `Refresh`, app entries from `/api/catalog/apps`, running surface
entries from `/api/surfaces`, `workspace 1`, mapped-surface status, and a clock.
Capture evidence for this state should still use the physical output path below.
The workspace model is intentionally conservative: `workspace-1` is the only
workspace, activation confirms that workspace, and the route reports mapped work
surface count until compositor-level multi-workspace layout exists.
The first launch-loop scenario uses `example-browser` as a fixture app: it
launches a compositor-managed toplevel surface, records running-state readback,
focuses the surface, captures the physical output while the app is visible, then
closes the surface and verifies stale running state is removed.

## Evidence Mapping

`EvidencePacket` fields map as follows:

| Packet field | den-k8 source |
| --- | --- |
| `scenario` | Stable live-check name, such as `den-k8-shell-http-installed-service` |
| `capturedAtUnixMillis` | Runner timestamp in Unix milliseconds |
| `visualStatus` | Capture inspection status: `visible`, `blank`, or `unknown` |
| `captureClassification` | Evidence ladder result |

Classification mapping:

| Live observation | `visualStatus` | `captureClassification` |
| --- | --- | --- |
| Shell route/service/socket only, no capture | `unknown` | `insufficient_mapped_only` |
| Valid installed JSON claim route, no capture | `unknown` | `insufficient_mapped_only` |
| Layer-shell content commit metadata without frame/capture | `unknown` | `content_committed` |
| Presented frame metadata without capture | `unknown` | `frame_presented` |
| Output capture with `visual_inspection.status == visible` and expected shell pixels | `visible` | `capture_visible` |
| Capture JSON with `visual_inspection.status == visible`, used as imported evidence | `visible` | `capture_visible` |
| Capture JSON with `visual_inspection.status == blank` | `blank` | `blank_capture_failure` |
| Surface absent or capture unavailable for a visibility claim | `unknown` | `not_visible` |

Mapped visibility alone is insufficient. Blank captures fail even when a
surface is mapped.

Current visible-shell scenario packets:

| Scenario | Meaning |
| --- | --- |
| `den-k8-installed-service-capture` | General installed shell output capture, used for dock/panel visibility. |
| `den-k8-shell-launch-visible` | Launch-loop capture taken after an app surface is running and focused. |
| `den-k8-native-launch-visible` | Governed native app launch capture taken after an allowlisted installed app is running and focused. |
| `den-k8-structured-layout-visible` | Structured layout capture taken after two or more native apps are running and focusable. |
| `den-k8-overlay-labels-visible` | Agent overlay capture taken after native app focus changes with labels and bounds visible. |

## Failure Taxonomy

Live checks fail closed:

| Failure | Meaning |
| --- | --- |
| `service` check failure | Installed unit is inactive or systemd cannot report it |
| `shell-ui` check failure | Shell route is unavailable or does not return shell HTML |
| `compositor` check failure | Required Unix socket is missing, wrong type, or cannot be connected |
| `capture` blank | Capture transport worked but visual output is blank |
| `capture` unavailable | A claim requiring visual evidence cannot be closed |
| invalid response | Installed service returned data the runner cannot parse |

Reports should be actionable without reading predecessor logs or agora-os
internals. If a needed boundary is absent, create an agora-de follow-up for the
installed-service interface or an agora-os follow-up when the missing piece is
fundamental OS/runtime behavior.

## Claim Checklist

Order live claim closure by risk:

1. Service liveness: required systemd units are active.
2. Shell availability: the installed shell HTML route returns HTTP 200.
3. Compositor bridge availability: required Unix sockets exist and accept a
   connection.
4. Surface lifecycle: installed service exposes surface state sufficient to
   prove mapped/focused/input-denied projections.
5. App catalog route: installed shell route returns stable catalog data.
6. Work surface controls: installed shell exposes mapped/focused/denied-input
   view data.
7. Audit, escalations, and agent health: installed service exposes typed
   boundary projections.
8. Capture/readback: capture JSON proves visibility or fails with a classified
   packet.

Items 4 through 8 may be blocked until the installed service exposes the
needed interface. Do not fill the gap with VM orchestration in agora-de.

## Running

Run the default installed-service checks:

```bash
./harness/live/check-den-k8.py
```

Run with capture evidence:

```bash
AGORA_DE_LIVE_CAPTURE_JSON=/path/to/capture.json ./harness/live/check-den-k8.py
```

Run with optional installed-service route claims:

```bash
AGORA_DE_LIVE_CATALOG_URL=http://127.0.0.1:7780/api/catalog/apps \
AGORA_DE_LIVE_SURFACES_URL=http://127.0.0.1:7780/api/surfaces \
AGORA_DE_LIVE_WORKSPACES_URL=http://127.0.0.1:7780/api/workspaces \
AGORA_DE_LIVE_OPERATOR_STATUS_URL=http://127.0.0.1:7780/api/operator/status \
./harness/live/check-den-k8.py
```

These route checks prove installed model/route shape. They do not close visual
claims without capture/readback evidence.

Run with compositor surface readback:

```bash
AGORA_DE_LIVE_SURFACE_APP_ID=io.agorade.ShellPanel \
AGORA_DE_LIVE_SURFACE_ROLE=panel \
./harness/live/check-den-k8.py
```

Require the selected surface to have presented a frame:

```bash
AGORA_DE_LIVE_REQUIRE_FRAME=1 \
AGORA_DE_LIVE_SURFACE_APP_ID=io.agorade.ShellPanel \
AGORA_DE_LIVE_SURFACE_ROLE=panel \
./harness/live/check-den-k8.py
```

Content-commit readback is stronger than mapped-surface JSON and weaker than
frame-presented metadata. Frame readback is stronger than content commits and
weaker than capture.

For layer-shell surfaces, `frame_count: 0` currently means the bridge has not
observed a frame-presented signal. It does not prove the monitor is blank; the
GTK4 panel has physically painted with this counter still at zero. A layer-shell
surface with `content_commit_count > 0` and `frame_count: 0` proves the client
committed content, not that the compositor presented it. Prefer physical output
capture once the compositor exposes the active monitor.

The first output-discovery step in the compositor bridge is intentionally
conservative: `compositorctl output list` may report a physical output with
`mode: physical_surface_readback` when mapped surfaces already carry an
`output_id` such as `HDMI-A-1`. That discovers the monitor identity but is not
yet image capture.

Require physical output capture evidence for the run to pass:

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

This invokes `compositorctl output capture --name HDMI-A-1 --export`, reads the
artifact PNG, and emits `capture_visible` only when the compositor inspection is
visible and the pixel classifier finds the expected light shell background,
accent line, dock/panel dark region, and text-like foreground pixels.

Task 4153 live evidence produced
`/run/agent-os/artifacts/den-k8-live-4153/output-capture-1783170766443939863-3/output-capture-1783170766443939863-3.png`
with `captureClassification: capture_visible` and
`pixelClassification.classification: expected_shell_visible`.

Run the launch/focus/close loop with capture evidence:

```bash
./harness/live/check-shell-loop.py \
  --base-url http://127.0.0.1:17780 \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-shell-loop \
  --require-capture
```

The launch-loop runner emits `agora-de.shell-loop-live.v1` with the same
classification vocabulary. Its capture packet uses
`scenario: den-k8-shell-launch-visible` and closes only when the physical output
capture is visible and the expected shell pixels are present while the launched
app is mapped. The runner waits briefly before capture so the dock's polling
loop can reflect the running-app entry in the visible panel.

Run the governed native app launch loop with capture evidence:

```bash
./harness/live/check-native-launch.py \
  --base-url http://127.0.0.1:17780 \
  --app-id Alacritty.desktop \
  --expected-app-id Alacritty \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-native-launch \
  --require-capture
```

The native-launch runner emits `agora-de.native-launch-live.v1`. It fails closed
if `/usr/local/bin/compositorctl` is selected, requires the target app to be
launchable in `/api/catalog/apps`, launches through `/api/catalog/launch`,
verifies compositor-backed surface readback and focus, captures the physical
output, closes the surface, and verifies stale cleanup.

Run the structured layout evidence loop with capture evidence:

```bash
./harness/live/check-structured-layout.py \
  --base-url http://127.0.0.1:17780 \
  --app-id Alacritty.desktop \
  --app-id foot.desktop \
  --expected-app-id Alacritty \
  --expected-app-id foot \
  --expected-zone primary \
  --expected-zone secondary \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-structured-layout \
  --require-capture
```

The structured-layout runner emits `agora-de.structured-layout-live.v1`. It
uses the installed shell/compositor path only: app catalog launch, surface
focus/close actions, `/api/layout`, and `/api/layout/action`. It distinguishes
`visibility`, `focus`, `occlusion-overlap`, `capture`, and `cleanup` failures.
Zone assignment may pass as backend-unsupported only when the final layout still
proves either distinct expected zones or non-overlapping geometry.

Run the planner-backed master-stack layout loop with capture evidence:

```bash
./harness/live/check-planner-layout.py \
  --base-url http://127.0.0.1:17780 \
  --app-id Alacritty.desktop \
  --app-id foot.desktop \
  --app-id firefox.desktop \
  --expected-app-id Alacritty \
  --expected-app-id foot \
  --expected-app-id firefox \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-planner-layout \
  --require-capture
```

The planner-layout runner emits `agora-de.planner-layout-live.v1`. It launches
three or more native apps, builds a backend-neutral `master_stack` plan from
the compositor output work area, sends planned rectangles through
`agora-de-compositorctl surface assign-zone --x --y --width --height`, verifies
backend acknowledgement through `layout get`, checks non-overlap, optionally
captures the physical output, and closes launched surfaces. Failures are split
into `planner-mismatch`, `backend-placement`, `focus-order`, `capture`, and
`cleanup` so planner math, backend application, evidence capture, and stale
cleanup can be diagnosed independently.

Run the agent-visible overlay label loop with capture evidence:

```bash
./harness/live/check-overlay-labels.py \
  --base-url http://127.0.0.1:17780 \
  --app-id Alacritty.desktop \
  --app-id foot.desktop \
  --expected-app-id Alacritty \
  --expected-app-id foot \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-overlay-labels \
  --require-capture
```

The overlay-labels runner emits `agora-de.overlay-labels-live.v1`. It requires
the installed `io.agorade.ShellOverlay` layer-shell surface, verifies the
`?surface=overlay` route has layout-driven label and bounds hooks, launches two
or more native work surfaces, proves stable layout labels and geometry, focuses
each target, captures after focus changes, and closes launched surfaces.
Failures are split into `overlay-route`, `overlay-surface`, `layout-labels`,
`focus`, `capture`, and `cleanup`.

Validate desktop-entry catalog import separately from the visible fixture launch
service by running a temporary shellui with `--catalog-provider desktop_entries`
and checking it with:

```bash
./harness/live/check-installed-catalog.py \
  --catalog-url http://127.0.0.1:17782/api/catalog/apps \
  --min-apps 1 \
  --require-all-nonlaunchable \
  --require-disabled-codes \
  --require-disabled-reasons
```

The runner emits `agora-de.installed-catalog-live.v1`. With native launch
disabled or no allowlist, imported installed entries must be visible in the
catalog and non-launchable in shellui with stable `disabledCode` values. With
`structured_compositorctl` enabled, only explicitly allowlisted desktop-entry
ids such as `Alacritty.desktop` should become launchable. On the current
den-k8 development install, `AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST=*` is
used so every installed entry that can be prepared by the structured argv
launcher is launchable.

Structured native launch evidence for task 4176 used the agora-de
`go/cmd/compositorctl` binary built to `/home/agent/.local/bin/agora-de-compositorctl`.
The direct launch command:

```bash
/home/agent/.local/bin/agora-de-compositorctl launch \
  --arg /usr/bin/alacritty \
  --arg --title \
  --arg AgoraNativeSmoke \
  --env HOME=/home/agent \
  --env USER=agent \
  --env LOGNAME=agent \
  --env PATH=/usr/local/bin:/usr/bin:/bin \
  --env XDG_RUNTIME_DIR=/run/user/1001 \
  --env WAYLAND_DISPLAY=wayland-1 \
  --env DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus \
  --env LANG=en_US.UTF-8 \
  --cwd /home/agent \
  --uid 1001 \
  --gid 1002 \
  --session-token session-native-smoke \
  --audit-correlation-id native-smoke \
  --output HDMI-A-1 \
  --wait-surface \
  --wait-timeout-ms 7000
```

returned `status: launched`, `surface_id: view-49`, app id `Alacritty`, and
title `AgoraNativeSmoke`.

The shellui sidecar launch command:

```bash
go run ./cmd/shellui \
  --listen 127.0.0.1:17783 \
  --catalog-provider desktop_entries \
  --desktop-entry-roots /usr/share/applications:/home/agent/.local/share/applications \
  --surface-provider fixture \
  --compositorctl /home/agent/.local/bin/agora-de-compositorctl \
  --native-launch-provider structured_compositorctl \
  --native-launch-allowlist Alacritty.desktop \
  --native-launch-uid 1001 \
  --native-launch-gid 1002 \
  --native-launch-session-token session-native-shell \
  --native-launch-output HDMI-A-1 \
  --native-launch-home /home/agent
```

exposed 59 installed apps with only `Alacritty.desktop` launchable.
`POST /api/catalog/launch {"appId":"Alacritty.desktop"}` returned
`launchId: launch-1783217014714420479`, `surfaceId: view-51`, and
`status: launched`. Surface readback saw `view-51` as app id `Alacritty`,
visible on `HDMI-A-1`. Physical output capture produced
`/run/agent-os/artifacts/unscoped/output-capture-1783217024425646244-14/output-capture-1783217024425646244-14.png`
with `visual_inspection.status: visible`.

Imported capture JSON remains supported:

```bash
AGORA_DE_LIVE_REQUIRE_CAPTURE=1 \
AGORA_DE_LIVE_CAPTURE_JSON=/path/to/capture.json \
./harness/live/check-den-k8.py
```

Useful overrides:

- `AGORA_DE_LIVE_SHELL_URL`
- `AGORA_DE_LIVE_COMPOSITORCTL`

The live compositor bridge daemon is installed from agora-de with:

```bash
sudo /home/dev/agora-de/deploy/compositor/install-compositor-bridge-service.sh
```

After installation, `systemctl cat compositor-bridge.service` should show
`ExecStart=/usr/local/bin/agora-de-compositor-bridge`. The Wayfire plugin is
still provided by the current Wayfire session and remains the active backend
runtime dependency.
- `AGORA_DE_LIVE_SYSTEMD_UNITS`
- `AGORA_DE_LIVE_SOCKETS`
- `AGORA_DE_LIVE_CAPTURE_JSON`
- `AGORA_DE_LIVE_REQUIRE_CAPTURE`
- `AGORA_DE_LIVE_COMPOSITORCTL`
- `AGORA_DE_LIVE_SURFACE_APP_ID`
- `AGORA_DE_LIVE_SURFACE_ROLE`
- `AGORA_DE_LIVE_REQUIRE_FRAME`
- `AGORA_DE_LIVE_CATALOG_URL`
- `AGORA_DE_LIVE_SURFACES_URL`
- `AGORA_DE_LIVE_WORK_CONTROLS_URL`
- `AGORA_DE_LIVE_WORKSPACES_URL`
- `AGORA_DE_LIVE_OPERATOR_STATUS_URL`
- `AGORA_DE_LIVE_TIMEOUT_SECONDS`

This harness belongs under `harness/live/` because it checks an installed
environment. It should not be wired into `check-all.sh`.

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
| Compositor/event bus | `/run/agent-os/bus.sock` | Unix socket exists and accepts connection |
| Compositor bridge | `/run/agent-os/compositor-bridge.sock` | Unix socket exists and accepts connection |
| Compositor control | `/run/agent-os/compositor-control.sock` | Unix socket exists and accepts connection |
| Capture artifacts | `/run/agent-os/captures` | capture JSON supplied to the runner |

The shell route currently proves installed shell availability, not visual
correctness. Visual claims need capture evidence.

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
| Presented frame metadata without capture | `unknown` | `frame_presented` |
| Capture JSON with `visual_inspection.status == visible` | `visible` | `capture_visible` |
| Capture JSON with `visual_inspection.status == blank` | `blank` | `blank_capture_failure` |
| Surface absent or capture unavailable for a visibility claim | `unknown` | `not_visible` |

Mapped visibility alone is insufficient. Blank captures fail even when a
surface is mapped.

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

Require capture evidence for the run to pass:

```bash
AGORA_DE_LIVE_REQUIRE_CAPTURE=1 \
AGORA_DE_LIVE_CAPTURE_JSON=/path/to/capture.json \
./harness/live/check-den-k8.py
```

Useful overrides:

- `AGORA_DE_LIVE_SHELL_URL`
- `AGORA_DE_LIVE_SYSTEMD_UNITS`
- `AGORA_DE_LIVE_SOCKETS`
- `AGORA_DE_LIVE_CAPTURE_JSON`
- `AGORA_DE_LIVE_REQUIRE_CAPTURE`
- `AGORA_DE_LIVE_TIMEOUT_SECONDS`

This harness belongs under `harness/live/` because it checks an installed
environment. It should not be wired into `check-all.sh`.

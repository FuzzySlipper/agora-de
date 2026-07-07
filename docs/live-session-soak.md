# Live Session Soak

The live-session soak track moves Agora DE evidence beyond point-in-time
installed-service checks. It is intentionally opt-in and host-facing: it runs
against the installed service on the current Linux session, not a VM, and it is
not part of normal CI.

## Scope

The bounded soak suite exercises:

- repeated native launch, focus, minimize, restore, and close cycles;
- launcher and status popup open/close cycles;
- workspace activation, including multi-workspace switching when the backend
  reports more than one workspace;
- overlay route and installed overlay surface health during churn;
- optional service restart probes through an explicit runner-supplied command;
- process and memory samples for Agora/Wayfire processes;
- surface, layout, workspace, operator-status, journal, and optional capture
  evidence.

The harness records evidence rather than pretending to be a full endurance
runner. Normal settings should stay short enough to run intentionally during
polish work. Longer cycles are useful before deployment updates, but they
should remain a live evidence action rather than a CI gate.

## Evidence Contract

`harness/live/check-live-session-soak.py` emits
`agora-de.live-session-soak.v1` JSON on stdout. The output contains:

- `checks`: preflight, cycle, capture, restart, drift, and cleanup pass/fail
  records;
- `samples`: route/process snapshots at initial, per-cycle, optional restart,
  and final phases;
- `events`: detailed launch, focus, popup, workspace, and restart actions;
- `journals`: recent user shell and compositor bridge journal tails;
- `evidencePackets`: optional physical output capture packets using the
  existing capture vocabulary;
- `summary`: total pass/fail counts.

When `--artifact-dir` is supplied, the same run also writes:

- `summary.json`;
- `samples.jsonl`;
- `journal-user-shellui.log`;
- `journal-user-panel.log`;
- `journal-user-overlay.log`;
- `journal-system-bridge.log`;
- `capture-packets.json` when capture evidence exists.

## Failure Boundaries

The soak suite separates product instability from harness cleanup issues:

- launch, focus, minimize, restore, workspace, popup, overlay, restart, and
  capture failures are product or installed-service failures unless the route
  is absent by design;
- `state-drift` means mapped work surfaces disappeared from layout state, or
  shell popups remained after their close cycle;
- `cleanup` means the harness could not remove surfaces it opened, or final
  state still showed drift;
- journal command failures are recorded as evidence but do not fail the soak by
  themselves, because journal access can vary by service/user scope.

The harness only closes surfaces that it opened during the current run. If a
native launch reuses an existing surface, the runner restores/focuses it and
leaves it open.

## Running

Default short installed-service soak:

```bash
./harness/live/check-live-session-soak.py \
  --base-url http://127.0.0.1:17780 \
  --cycles 2 \
  --artifact-dir /tmp/agora-de-live-session-soak
```

Run with physical output capture:

```bash
./harness/live/check-live-session-soak.py \
  --base-url http://127.0.0.1:17780 \
  --cycles 2 \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-live-session-soak \
  --require-capture \
  --artifact-dir /tmp/agora-de-live-session-soak
```

Run with an explicit service restart probe:

```bash
./harness/live/check-live-session-soak.py \
  --base-url http://127.0.0.1:17780 \
  --cycles 2 \
  --restart-command '/usr/local/sbin/agora-de-compositor-bridge-admin restart-bridge' \
  --artifact-dir /tmp/agora-de-live-session-soak
```

Use a longer intentional soak by increasing `--cycles`. Keep app choices
explicit when testing host-specific launch behavior:

```bash
./harness/live/check-live-session-soak.py \
  --app-id Alacritty.desktop \
  --expected-app-id Alacritty \
  --app-id firefox.desktop \
  --expected-app-id firefox \
  --cycles 10 \
  --artifact-dir /tmp/agora-de-live-session-soak-long
```

## Closeout Criteria

The live-session soak track closes when:

- the harness produces a pass/fail summary and durable artifacts on den-k8;
- process/memory samples, surface/layout drift, journal tails, and capture
  health are visible in the run output;
- recurring product issues are fixed or split into concrete follow-up tasks;
- cleanup-only or host-access issues are identified as harness/recovery work
  rather than blended into product instability.

## 2026-07-07 Task 4571 Evidence

The initial capture-backed installed-service soak ran on den-k8 with:

```bash
./harness/live/check-live-session-soak.py \
  --base-url http://127.0.0.1:17780 \
  --cycles 1 \
  --output-name HDMI-A-1 \
  --output-capture-session den-k8-live-session-soak-4571 \
  --require-capture \
  --artifact-dir /tmp/agora-de-live-session-soak-4571-capture \
  --timeout-seconds 8
```

Result:

- `10 passed / 0 failed`;
- artifacts written under `/tmp/agora-de-live-session-soak-4571-capture`;
- capture packet written to `capture-packets.json`;
- physical output capture:
  `/run/agent-os/artifacts/den-k8-live-session-soak-4571/output-capture-1783465385672928500-1/output-capture-1783465385672928500-1.png`;
- capture classified as `capture_visible` with
  `pixelClassification.classification: expected_shell_visible`;
- cycle exercised workspace activation, shell status popup, shell launcher
  popup, Alacritty launch/focus/minimize/restore/close, state-drift cleanup,
  process samples, and journal tails.

The run still showed recurring compositor bridge broken-pipe journal lines
during short-lived client interactions, so task 4751 tracks classification or
log reduction. Task 4752 tracks longer-run memory threshold interpretation on
top of the raw process samples.

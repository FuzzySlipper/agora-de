# Cutover Checklist

agora-de can wait until it is solid. Cutover does not require predecessor
runtime compatibility, shim deployment, or legacy fallback modes.

## Local Deterministic Gate

Required before any cutover candidate:

```bash
./harness/ci/check-all.sh
```

This covers ownership, Rust, Go, TypeScript, contract drift, live-evidence
fixtures, compositor fixtures, Wayfire fixtures, chrome spike fixtures, and
docs command checks.

## den-k8 Installed-Service Gate

Required before user-visible claims are closed:

```bash
./harness/live/check-den-k8.py
```

For visual claims, run with capture evidence:

```bash
AGORA_DE_LIVE_REQUIRE_CAPTURE=1 \
AGORA_DE_LIVE_CAPTURE_JSON=/path/to/capture.json \
./harness/live/check-den-k8.py
```

VM orchestration remains outside agora-de. If fundamental OS/runtime VM coverage
is needed, split that work to agora-os.

## User-Visible Claim Evidence

| Claim | Required scenario | Closure evidence |
| --- | --- | --- |
| Installed shell route is available | `den-k8-shell-http-installed-service` | Live runner passes shell route check; this is availability only, not visual closure. |
| Desktop shell surface controls are coherent | `desktop-shell-surface-controls-model-fixture` then `den-k8-desktop-shell-dock-visible` | TS model fixture passes and live capture packet is `capture_visible` or stronger readback evidence. |
| App launcher catalog is visible and populated | `desktop-shell-surface-controls-model-fixture` plus catalog route check | Catalog route returns valid app data; visual shell claim still needs nonblank capture. |
| Operator console boundary projections are coherent | `operator-console-boundary-projections-model-fixture` | Audit, escalation, and agent-health model fixtures pass; live closure waits for installed service route/capture. |
| Capture/readback detects blank output | `den-k8-installed-service-capture` | Blank capture produces `blank_capture_failure` and fails closed. |
| Native layer-shell chrome is promoted | `den-k8-layer-shell-dock-visible` | GTK4/WebKitGTK spike must be promoted by evidence; until then it remains an inspectable candidate. |

Mapped-only evidence stays insufficient for user-visible shell closure.

## Deploy Productization Gate

`deploy/` is for productization only:

- packaging service units;
- install paths;
- release operator notes;
- artifact wiring.

Do not stage source code, experiments, predecessor adapters, or fallback modes
under `deploy/`.

## Hold Criteria

Hold cutover when:

- the local full gate fails;
- den-k8 installed-service checks fail;
- a user-visible claim has only mapped/model evidence and no inspected capture;
- capture evidence is blank or stale;
- a needed installed-service boundary is missing;
- deploy material requires source-code staging or a legacy runtime shim.

Rollback is a release decision to keep the predecessor deployment in place until
agora-de is ready. It is not a reason to add compatibility code to agora-de.

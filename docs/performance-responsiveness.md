# Performance And Responsiveness

This note records the post-northstar responsiveness baseline for the installed
Agora DE service on den-k8. The evidence is intentionally live-host oriented:
the harnesses talk to the installed shell service and compositor bridge, not a
VM or mock desktop.

## Evidence Harness

Use the responsiveness baseline harness for repeatable route and action timing:

```bash
./harness/live/check-responsiveness-baseline.py \
  --base-url http://127.0.0.1:17780 \
  --app-id Alacritty.desktop \
  --expected-app-id Alacritty
```

The harness emits `agora-de.responsiveness-baseline-live.v1`, records min, p50,
p95, max, and sample timings, then closes shell popups and any native app it
opened.

Shellui also exposes a low-noise in-memory diagnostic summary:

```text
GET /api/diagnostics/timing
GET /api/operator/status  # includes timing
```

The diagnostic schema is `agora-de.shell-timing.v1`. It records bounded recent
samples per route/action, status-code classes, and category/backend labels such
as `shell_http`, `compositor_action`, `launch_action`, `compositorctl`,
`native_launch`, `catalog`, `operator`, and `theme`.

## 2026-07-07 Baseline

Baseline was captured before tuning with task 4617:

| Path | p50 | p95/max | Note |
| --- | ---: | ---: | --- |
| `/api/theme` | 1.055 ms | 1.197 ms p95 | Theme route is comfortably cheap. |
| `/api/catalog/apps` | 1.440 ms | 1.984 ms p95 | Catalog projection is comfortably cheap. |
| `/api/surfaces` | 4.261 ms | 5.421 ms p95 | Compositor-backed read path is healthy. |
| `/api/layout` | 3.933 ms | 4.562 ms p95 | Layout projection is healthy. |
| `/api/workspaces` | 6.562 ms | 6.870 ms p95 | Workspace projection is healthy. |
| `/api/operator/status` | 26.624 ms | 28.908 ms p95 | Operator route does service/socket/output checks. |
| shell status launch HTTP | 607.513 ms | 607.513 ms max | GTK/WebKit layer-shell process startup dominates. |
| shell launcher launch HTTP | 603.897 ms | 603.897 ms max | GTK/WebKit layer-shell process startup dominates. |
| native Alacritty launch HTTP | 205.266 ms | 205.266 ms max | Compositorctl surface polling was a visible component. |
| native launch-to-observed | 3.131 ms | 3.131 ms max | Surface was available immediately after HTTP returned. |
| native focus | 6.446 ms | 6.446 ms max | Healthy. |
| native close | 114.379 ms | 114.379 ms max | Close acknowledgement plus state disappearance wait. |

## 2026-07-07 Tuning Result

Task 4619 reduced the compositorctl launch surface poll interval from 200 ms to
50 ms while preserving the same mapped-surface evidence and timeout behavior.
The installed shell was configured to use
`/home/agent/.local/bin/agora-de-compositorctl`; that client was rebuilt from
the tuned code before the live evidence runs.

| Path | Baseline | Tuned run 1 | Tuned run 2 | Result |
| --- | ---: | ---: | ---: | --- |
| native Alacritty launch HTTP | 205.266 ms | 104.073 ms | 104.153 ms | Improved by about 100 ms. |
| native launch-to-observed | 3.131 ms | 2.279 ms | not recorded in summary | Still immediate after HTTP return. |
| native focus | 6.446 ms | 5.085 ms | 5.977 ms | Healthy. |
| native close | 114.379 ms | 111.487 ms | not recorded in summary | Essentially unchanged. |
| shell status launch HTTP | 607.513 ms | 555.143 ms | 504.189 ms | Some improvement, still toolkit/process-startup dominated. |
| shell launcher launch HTTP | 603.897 ms | 653.788 ms | 604.600 ms | No reliable improvement; classify as toolkit/process-startup dominated. |

Both tuned harness runs passed with `22 passed / 0 failed / 0 skipped` and no
stale shell popup surfaces.

## Targets

Near-term targets are deliberately practical rather than aspirational:

| Interaction | Target | Current status |
| --- | ---: | --- |
| Cheap shell read routes | p95 under 15 ms | Met for theme, catalog, surfaces, layout, workspaces. |
| Operator status | p95 under 50 ms | Met; keep service/socket checks bounded. |
| Native app launch HTTP | p50 under 150 ms for already-installed light apps | Met after poll tuning for Alacritty. |
| Native focus | p50 under 20 ms | Met. |
| Native close-to-absent | p50 under 150 ms | Met. |
| Shell popup launch | p50 under 500 ms if feasible | Not yet met consistently; GTK/WebKit layer-shell startup is the likely limit. |

## Follow-Up Direction

Keep measuring before tuning. The current evidence suggests the next useful
performance work is not route JSON speed, but popup/app startup and refresh
sequencing:

- use `/api/diagnostics/timing` to compare shell HTTP route cost with
  compositorctl-backed action cost after deployment;
- avoid extra post-action refreshes where the action result already includes
  enough state;
- investigate long-lived or prewarmed shell popup surfaces only if usability
  demands faster launcher/status opening, because that changes lifecycle
  behavior and should be designed explicitly;
- keep live harness cleanup restorative so performance testing does not leave
  stale windows for the human desktop session.

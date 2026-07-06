# Auto-Tiling WM Northstar Review

Status: reviewed during Den task 4373 on 2026-07-06.

Parent northstar: Den 4338, `Northstar: produce a deployed auto-tiling WM
usable by users and agents`.

Conclusion: do not close the northstar yet. The deployed WM is now usable and
well evidenced on den-k8, but the closeout criteria still require a rehearsed
existing-Linux install path and a few remaining WM hardening items.

## Evidence Baseline

Recent commits and evidence:

| Area | Evidence |
| --- | --- |
| Auto-layout loop and placement | `7fda7cf` added the deployed auto-tiling live harness; later fixes through `0ad856d` stabilized coherent auto-layout snapshots. |
| User controls | `1e4e60b` exposed shell WM controls; `97056de` polished target/settings/unsupported feedback. |
| Daily workflow | `1c5815e4` proved terminal/browser/file-manager workflow. Live command: `./harness/live/check-daily-wm-workflow.py --base-url http://127.0.0.1:17780 --app-id Alacritty.desktop --app-id foot.desktop --app-id firefox.desktop --app-id org.kde.dolphin.desktop --expected-app-id Alacritty --expected-app-id foot --expected-app-id firefox --expected-app-id org.kde.dolphin --output-name HDMI-A-1 --output-capture-session den-k8-daily-wm-workflow-4370 --require-capture`. Capture: `/run/agent-os/artifacts/den-k8-daily-wm-workflow-4370/output-capture-1783329942773572938-1/output-capture-1783329942773572938-1.png`. |
| Shell/chrome reliability | `33fd40a` moved the launcher to a chrome-owned layer popup; `7cd6d4f` added bridge fail-fast recovery for plugin reconnect churn. |
| Native app visibility | `688f214` fixed native app pixels and stale shell readback; task 4405 evidence distinguished mapped state from physical pixels. |
| Non-occluding diagnostics overlay | `c184bd3` added the GTK4/Cairo native overlay. Live command: `./harness/live/check-overlay-labels.py --base-url http://127.0.0.1:17780 --app-id Alacritty.desktop --app-id firefox.desktop --expected-app-id Alacritty --expected-app-id firefox --output-name HDMI-A-1 --output-capture-session den-k8-overlay-native-4422 --require-capture`. Captures: `/run/agent-os/artifacts/den-k8-overlay-native-4422-focus-1/output-capture-1783341965377690359-2/output-capture-1783341965377690359-2.png` and `/run/agent-os/artifacts/den-k8-overlay-native-4422-focus-2/output-capture-1783341971921981029-3/output-capture-1783341971921981029-3.png`. |
| CI | `./harness/ci/check-all.sh` passes on current main. GitHub `Agora DE CI` passed for `c184bd3e8849cbe7e2e9f2c4c61f31b10a466742`. |

## Checklist Assessment

| Northstar foundation | Status | Notes |
| --- | --- | --- |
| Backend-neutral Rust layout model owns planner semantics | Pass | `layout-model` fixtures and compositor contract checks are in CI. |
| Current compositor adapter applies planned rectangles and reports acknowledgement geometry | Pass | Wayfire bridge applies placements and publishes layout state; live harnesses check backend placement and mismatch classes. |
| Installed den-k8 deployment runs continuously without VM-only validation or old shims | Pass for den-k8 | All live validation targets the installed service; no legacy agora-os shell shim was added. |
| Auto-layout reacts to map, unmap, focus, close, mode, and settings changes | Pass | Auto-layout worker and live checks cover map/close/focus recovery. |
| Lifecycle rules cover normal apps, browsers, file manager, shell/transient surfaces, and stale cleanup | Mostly pass | Terminal, Firefox, Dolphin, shell launcher/status, stale layer-shell cleanup, and close recovery are covered. Dialog/transient behavior is classified conservatively; deeper transient policy can remain future refinement. |
| User controls expose common WM actions without CLI | Mostly pass | Panel controls cover focus, promote, move, float/tile, reset, settings, close, and honest unsupported states. Fullscreen/maximize/minimize still return `backend_unsupported` rather than performing a compositor-owned state change. |
| Agent controls expose stable ids, labels, app ids, bounds, zone membership, focus order, results, and recovery | Pass | `/api/layout`, `/api/surfaces`, compositorctl, overlay labels, and recovery helpers cover the current single-workspace model. |
| Visual evidence includes always-available non-occluding overlay/bounds model | Pass | Native overlay is opt-in but installed and proven non-occluding; default desktop remains usable with overlay disabled. |
| Theming/presentation remains centralized and separate from WM plumbing | Pass | Theme tokens and docs are centralized; WM plumbing does not own visual styling. |
| Live harnesses prove non-overlap, placement, focus/order, controls, cleanup, capture, restart/recovery | Pass with known launch caveat | Auto-tiling, daily workflow, overlay, native launch, planner, and recovery checks exist. Firefox can produce a visible surface after a launch timeout, which needs launch lifecycle cleanup for agent reliability. |
| Failure handling is classified and observable | Pass | Current harnesses distinguish launch, visibility, planner mismatch, backend placement, occlusion, focus/order, shell action, agent action, overlay, capture, restart, cleanup, stale, unsupported, and unavailable states. |
| Documentation records user model, agent model, backend boundary, deployment, and evidence | Pass with install rehearsal gap | Docs are present, but the non-agora-os existing-Linux install path still needs rehearsal. |

## Remaining Gaps

1. Existing-Linux install rehearsal is not complete.
   Den task 4372 already tracks this. It should remain the next required
   northstar task before closeout.

2. Fullscreen/maximize/minimize are honest but not implemented as compositor
   state changes.
   Current behavior is acceptable for daily tiled use because the UI reports
   `backend_unsupported`, but it does not fully satisfy the northstar control
   list.

3. Multi-workspace compositor placement is not implemented.
   The current single `workspace-1` model is deterministic and useful, but the
   northstar names workspace operations as part of the target WM behavior.

4. Native launch completion semantics are still too coarse for apps that reuse
   an existing process or create a window after the shell request times out.
   Firefox was observed in task 4422 evidence as producing a mapped visible
   surface even after a `timed_out` launch response.

5. Final closeout should rerun the full installed-service evidence suite after
   the above gaps are either implemented or explicitly scoped out.

## Next Decision

Keep Den 4338 open. Do not mark the northstar done until Den 4372 and the
follow-up WM hardening tasks are complete or deliberately scoped out with fresh
evidence.

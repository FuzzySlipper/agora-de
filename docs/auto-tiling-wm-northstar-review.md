# Auto-Tiling WM Northstar Review

Status: closed during Den task 4452 on 2026-07-07.

Parent northstar: Den 4338, `Northstar: produce a deployed auto-tiling WM
usable by users and agents`.

Conclusion: close the northstar. The installed den-k8 service now proves an
existing-Linux agora-de desktop environment with deterministic tiling, native
app launch, taskbar controls, workspace controls, overlay evidence, recovery
helpers, and no runtime dependency on agora-os governance or predecessor shell
shims.

## Closeout Evidence

All commands below were run against the installed service on den-k8 after
rebuilding and restarting the agora-de compositor bridge and shell user
services from the current checkout.

| Area | Result | Evidence |
| --- | --- | --- |
| Full repo CI | Pass | `./harness/ci/check-all.sh` returned `agora-de check-all: OK`. |
| Native app launch | 10 passed, 0 failed | `./harness/live/check-native-launch.py --base-url http://127.0.0.1:17780 --app-id Alacritty.desktop --expected-app-id Alacritty --output-name HDMI-A-1 --output-capture-session den-k8-northstar-4452-native --require-capture`. Capture: `/run/agent-os/artifacts/den-k8-northstar-4452-native/output-capture-1783410126591400873-2/output-capture-1783410126591400873-2.png`. |
| Layout commands | 17 passed, 0 failed | `./harness/live/check-layout-commands.py --base-url http://127.0.0.1:17780 --app-id Alacritty.desktop --app-id foot.desktop --expected-app-id Alacritty --expected-app-id foot --output-name HDMI-A-1 --output-capture-session den-k8-northstar-4452-layout-commands --require-capture`. Capture: `/run/agent-os/artifacts/den-k8-northstar-4452-layout-commands/output-capture-1783410140553743134-3/output-capture-1783410140553743134-3.png`. |
| Planner layout | 15 passed, 0 failed | `python3 ./harness/live/check-planner-layout.py --base-url http://127.0.0.1:17780 --app-id Alacritty.desktop --app-id foot.desktop --app-id firefox.desktop --expected-app-id Alacritty --expected-app-id foot --expected-app-id firefox --output-name HDMI-A-1 --output-capture-session den-k8-northstar-4452-planner --require-capture`. Capture: `/run/agent-os/artifacts/den-k8-northstar-4452-planner/output-capture-1783410157755866091-4/output-capture-1783410157755866091-4.png`. |
| Auto-tiling WM suite | 20 passed, 0 failed | `./harness/live/check-auto-tiling-wm.py --base-url http://127.0.0.1:17780 --app-id Alacritty.desktop --app-id foot.desktop --app-id firefox.desktop --expected-app-id Alacritty --expected-app-id foot --expected-app-id firefox --output-name HDMI-A-1 --output-capture-session den-k8-northstar-4452-auto --require-capture`. Capture: `/run/agent-os/artifacts/den-k8-northstar-4452-auto/output-capture-1783410029554235811-1/output-capture-1783410029554235811-1.png`. |
| Daily workflow | 27 passed, 0 failed | `./harness/live/check-daily-wm-workflow.py --base-url http://127.0.0.1:17780 --app-id Alacritty.desktop --app-id foot.desktop --app-id firefox.desktop --app-id org.kde.dolphin.desktop --expected-app-id Alacritty --expected-app-id foot --expected-app-id firefox --expected-app-id org.kde.dolphin --output-name HDMI-A-1 --output-capture-session den-k8-northstar-4452-daily --require-capture`. Capture: `/run/agent-os/artifacts/den-k8-northstar-4452-daily/output-capture-1783410193462605688-5/output-capture-1783410193462605688-5.png`. |
| Agent overlay | 12 passed, 0 failed | `./harness/live/check-overlay-labels.py --base-url http://127.0.0.1:17780 --app-id Alacritty.desktop --app-id firefox.desktop --expected-app-id Alacritty --expected-app-id firefox --output-name HDMI-A-1 --output-capture-session den-k8-northstar-4452-overlay --require-capture`. Captures: `/run/agent-os/artifacts/den-k8-northstar-4452-overlay-focus-1/output-capture-1783410207557543385-6/output-capture-1783410207557543385-6.png` and `/run/agent-os/artifacts/den-k8-northstar-4452-overlay-focus-2/output-capture-1783410214128878953-7/output-capture-1783410214128878953-7.png`. |
| Popup/taskbar stability | 9 passed, 0 failed | `./harness/live/check-popup-stability.py --base-url http://127.0.0.1:17780 --output-name HDMI-A-1 --output-capture-session den-k8-northstar-4452-popup --require-capture --samples-output /tmp/agora-de-4452-popup-samples.json`. Captures: `/run/agent-os/artifacts/den-k8-northstar-4452-popup-status/output-capture-1783410240508937731-8/output-capture-1783410240508937731-8.png` and `/run/agent-os/artifacts/den-k8-northstar-4452-popup-launcher/output-capture-1783410249343619718-9/output-capture-1783410249343619718-9.png`. |
| Default installed desktop | 11 passed, 0 failed | `./harness/live/check-den-k8.py --shell-url 'http://127.0.0.1:17780/shell/dist/desktop/?surface=dock' --systemd-units compositor-bridge.service --sockets /run/agent-os/compositor-bridge.sock,/run/agent-os/compositor-control.sock --catalog-url http://127.0.0.1:17780/api/catalog/apps --surfaces-url http://127.0.0.1:17780/api/surfaces --work-controls-url http://127.0.0.1:17780/api/work-surface-controls --workspaces-url http://127.0.0.1:17780/api/workspaces --operator-status-url http://127.0.0.1:17780/api/operator/status --surface-app-id io.agorade.ShellPanel --surface-role panel --output-name HDMI-A-1 --output-capture-session den-k8-northstar-4452-default --require-capture`. Capture: `/run/agent-os/artifacts/den-k8-northstar-4452-default/output-capture-1783410312980767601-11/output-capture-1783410312980767601-11.png`. |
| No-overlay desktop | 11 passed, 0 failed | Same `check-den-k8.py` route/service suite with `agora-de-shell-overlay.service` stopped, then restarted afterward. Capture: `/run/agent-os/artifacts/den-k8-northstar-4452-no-overlay/output-capture-1783410332481528705-12/output-capture-1783410332481528705-12.png`. |
| Recovery/admin path | Pass | `sudo -n /usr/local/sbin/agora-de-compositor-bridge-admin status` reported active; `sudo -n /usr/local/sbin/agora-de-kill-all --help` reported the durable cleanup command. |

## Checklist Assessment

| Northstar foundation | Status | Notes |
| --- | --- | --- |
| Backend-neutral Rust layout model owns planner semantics | Pass | `layout-model` fixtures and compositor contract checks are in CI. |
| Current compositor adapter applies planned rectangles and reports acknowledgement geometry | Pass | Wayfire bridge applies placements and publishes layout state; live harnesses check backend placement and mismatch classes. |
| Installed den-k8 deployment runs continuously without VM-only validation or old shims | Pass | All live validation targets the installed service on the existing Linux host; no legacy agora-os shell shim was added. |
| Existing-Linux install/update path is rehearsed | Pass | Task 4372 rehearsed the install path, and task 4452 rebuilt/restarted installed bridge and shell services before final evidence. |
| Auto-layout reacts to map, unmap, focus, close, mode, workspace, and settings changes | Pass | Auto-layout worker and live checks cover map/close/focus/restart recovery, command modes, planner geometry, and workspace activation. |
| Lifecycle rules cover normal apps, browsers, file manager, shell/transient surfaces, and stale cleanup | Pass | Terminal, Firefox, Dolphin, shell launcher/status, stale layer-shell cleanup, and close/relaunch recovery are covered. |
| User controls expose common WM actions without CLI | Pass | Panel controls cover focus, promote, move, float/tile, fullscreen, maximize, minimize, reset, settings, workspace activation, and close. Shell state routes now accept explicit enable/disable for reversible fullscreen/maximize/minimize checks. |
| Agent controls expose stable ids, labels, app ids, bounds, zone membership, focus order, results, and recovery | Pass | `/api/layout`, `/api/surfaces`, compositorctl, overlay labels, and recovery helpers cover the deployed model. |
| Visual evidence includes always-available non-occluding overlay/bounds model | Pass | Native overlay is installed and proven non-occluding; default desktop remains usable with overlay disabled. |
| Theming/presentation remains centralized and separate from WM plumbing | Pass | Theme tokens and docs are centralized; WM plumbing does not own visual styling. |
| Live harnesses prove non-overlap, placement, focus/order, controls, cleanup, capture, restart/recovery | Pass | Auto-tiling, daily workflow, overlay, popup stability, native launch, planner, command, default desktop, no-overlay desktop, and recovery/admin checks passed. |
| Failure handling is classified and observable | Pass | Current harnesses distinguish launch, visibility, planner mismatch, backend placement, occlusion, focus/order, shell action, agent action, overlay, capture, restart, cleanup, stale, unsupported, and unavailable states. |
| Documentation records user model, agent model, backend boundary, deployment, and evidence | Pass | `docs/deployed-wm-model.md`, this review, install/runbooks, and live-evidence docs record the current model. |

## Follow-Up Scope

Den 4338 can close. Follow-up work should move into post-northstar usability and
backend evolution tasks rather than blocking this foundation:

- richer minimized-window restore affordances in the taskbar;
- multi-output workspace policy;
- deeper transient/dialog policy beyond the current conservative classifier;
- theme development and taskbar visual tuning;
- longer-term Smithay/Rust backend evaluation.

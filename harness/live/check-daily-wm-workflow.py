#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import sys
import time
import urllib.error
import urllib.request


SCHEMA = "agora-de.daily-wm-workflow-live.v1"
SCENARIO = "den-k8-daily-wm-workflow-visible"
OLD_COMPOSITORCTL = pathlib.Path("/usr/local/bin/compositorctl")
PANEL_APP_IDS = {"io.agorade.ShellLauncher", "io.agorade.ShellStatus", "io.agorade.ShellOverlay"}


def main() -> int:
    parser = argparse.ArgumentParser(description="Check an installed-service daily WM workflow.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--overlay-app-id", default="io.agorade.ShellOverlay")
    parser.add_argument("--app-id", action="append", default=[], help="App id to launch; repeat for four daily-workflow apps.")
    parser.add_argument("--expected-app-id", action="append", default=[], help="Expected compositor app id matching each --app-id.")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-daily-wm-workflow")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.5)
    parser.add_argument("--require-capture", action="store_true")
    parser.add_argument("--timeout-seconds", type=float, default=12)
    args = parser.parse_args()

    structured = load_module("check-structured-layout.py", "agora_de_check_structured_layout")
    auto = load_module("check-auto-tiling-wm.py", "agora_de_check_auto_tiling_wm")
    overlay = load_module("check-overlay-labels.py", "agora_de_check_overlay_labels")

    app_ids = args.app_id or ["Alacritty.desktop", "foot.desktop", "firefox.desktop", "org.kde.dolphin.desktop"]
    expected_app_ids = args.expected_app_id or ["Alacritty", "foot", "firefox", "org.kde.dolphin"]
    checked_at = structured.unix_millis()
    checks = []
    evidence_packets = []
    workflow_events = []
    launched = []
    latest_layout = {}

    if len(app_ids) < 4:
        checks.append(failed("config", "daily WM workflow proof requires at least four --app-id values"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)
    if len(expected_app_ids) != len(app_ids):
        checks.append(failed("config", "--expected-app-id count must match --app-id count"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

    path_check = structured.check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

    try:
        shell_route = check_shell_controls_route(auto, args.base_url)
        checks.append(shell_route)
        if shell_route["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        catalog_check = auto.check_catalog(structured, args.base_url, app_ids)
        checks.append(catalog_check)
        if catalog_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        for app_id, expected_app_id in zip(app_ids, expected_app_ids):
            launch_check, item = auto.launch_app(structured, args.base_url, app_id, expected_app_id)
            checks.append(launch_check)
            if launch_check["status"] != "pass" or item is None:
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)
            launched.append(item)
            workflow_events.append({"action": "launch", "appId": app_id, "surfaceId": item["surfaceId"]})

        visibility = auto.wait_for_launched_surfaces(structured, args.base_url, active_launched(launched), args.timeout_seconds)
        checks.append(visibility)
        if visibility["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        set_mode = post_json_result(args.base_url + "/api/layout/action", {"action": "setMode", "mode": "zones"})
        checks.append(classify_shell_action("shell-action", "shell layout mode control accepted", set_mode, action="setMode"))
        if checks[-1]["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)
        workflow_events.append({"action": "setMode", "mode": "zones", "status": set_mode[0]})

        layout_check, latest_layout = wait_for_daily_layout(auto, args.base_url, active_launched(launched), args.timeout_seconds)
        checks.append(layout_check)
        if layout_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)
        for check in [
            auto.check_planner_state(latest_layout, active_launched(launched)),
            auto.check_backend_placement(latest_layout, active_launched(launched)),
            auto.check_occlusion(latest_layout, active_launched(launched)),
        ]:
            checks.append(check)
            if check["status"] != "pass":
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        focus_check, focus_events, latest_layout = exercise_focus_next_previous(auto, structured, args.base_url, active_launched(launched), args.timeout_seconds)
        workflow_events.extend(focus_events)
        checks.append(focus_check)
        if focus_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        promote_check, promote_events, latest_layout = exercise_promote_to_master(auto, structured, args.base_url, active_launched(launched), args.timeout_seconds)
        workflow_events.extend(promote_events)
        checks.append(promote_check)
        if promote_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        move_check, move_events, latest_layout = exercise_move_zone_recovery(auto, args.base_url, active_launched(launched), args.timeout_seconds)
        workflow_events.extend(move_events)
        checks.append(move_check)
        if move_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        float_check, float_events, latest_layout = exercise_float_tile(auto, args.base_url, active_launched(launched), args.timeout_seconds)
        workflow_events.extend(float_events)
        checks.append(float_check)
        if float_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        panel_check, panel_events = exercise_launcher_and_status(structured, args.base_url, args.timeout_seconds)
        workflow_events.extend(panel_events)
        checks.append(panel_check)
        if panel_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        restart_check, restart_events, latest_layout = exercise_close_relaunch_recovery(auto, structured, args.base_url, launched, args.timeout_seconds)
        workflow_events.extend(restart_events)
        checks.append(restart_check)
        if restart_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        overlay_check = auto.check_overlay_state(overlay, args.base_url, args.overlay_app_id, active_launched(launched), args.timeout_seconds)
        checks.append(overlay_check)
        if overlay_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)
        workflow_events.append({"action": "overlay", "surfaceId": overlay_check.get("overlaySurfaceId", ""), "labels": overlay_check.get("labels", {})})

        recovery_wait, latest_layout = wait_for_daily_layout(auto, args.base_url, active_launched(launched), args.timeout_seconds)
        checks.append(recovery_wait)
        if recovery_wait["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)
        recovery = check_daily_recovery(auto, latest_layout, active_launched(launched))
        checks.append(recovery)
        if recovery["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        if args.output_name:
            if args.capture_delay_seconds > 0:
                time.sleep(args.capture_delay_seconds)
            capture_check, packet = structured.capture_visible_structured_layout(
                args.compositorctl,
                args.output_name,
                args.output_capture_session,
                checked_at,
                auto.surface_ids(active_launched(launched)),
            )
            capture_check = dict(capture_check)
            if capture_check.get("status") == "pass":
                capture_check["name"] = "capture"
                capture_check["detail"] = "physical output capture shows deployed daily WM workflow"
            checks.append(capture_check)
            if packet:
                packet = dict(packet)
                packet["scenario"] = SCENARIO
                packet["surfaceIds"] = auto.surface_ids(active_launched(launched))
                evidence_packets.append(packet)
        elif args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)

        cleanup_checks = auto.cleanup_launched(structured, args.base_url, active_launched(launched), args.timeout_seconds)
        checks.extend(cleanup_checks)
    finally:
        for item in active_launched(launched):
            try:
                structured.post_json(args.base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
            except Exception:
                pass

    return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, workflow_events, latest_layout, checked_at)


def load_module(filename: str, module_name: str):
    path = pathlib.Path(__file__).with_name(filename)
    spec = importlib.util.spec_from_file_location(module_name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def check_shell_controls_route(auto, base_url: str) -> dict:
    route = auto.check_shell_controls_route(base_url)
    if route["status"] != "pass":
        return route
    body = get_text(base_url + "/shell/dist/desktop/?surface=dock")
    required = [
        'id="wm-controls"',
        'id="apps-button"',
        'id="operator-button"',
        "toggleApps",
        'launchApp("shell-status")',
        "focusRelative(-1)",
        "focusRelative(1)",
        "moveTargetToNextZone",
        "toggleTargetFloating",
    ]
    missing = [value for value in required if value not in body]
    if missing:
        return failed("shell-action", "dock route is missing daily workflow control hooks", missing=missing)
    return passed("shell-action", "dock route exposes daily WM workflow controls")


def exercise_focus_next_previous(auto, structured, base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, list[dict], dict]:
    layout = auto.wait_for_auto_layout(base_url, launched, timeout_seconds) or {}
    ordered = ordered_layout_surfaces(auto, layout, launched)
    if len(ordered) < 2:
        return failed("focus-order", "focus next/previous requires at least two manageable surfaces"), [], layout
    start = next((surface for surface in ordered if surface.get("focused")), ordered[0])
    start_index = ordered.index(start)
    events = []
    latest_layout = layout
    for action_name, delta in [("focusNext", 1), ("focusPrevious", -1)]:
        ordered = ordered_layout_surfaces(auto, latest_layout, launched)
        current = next((surface for surface in ordered if surface.get("focused")), start)
        index = ordered.index(current) if current in ordered else start_index
        target = ordered[(index + delta + len(ordered)) % len(ordered)]
        response = structured.post_json(base_url + "/api/layout/action", {"surfaceId": target["surfaceId"], "action": "promote"})
        if response.get("status") != "accepted":
            return failed("focus-order", f"{action_name} promote action was not accepted", response=response), events, latest_layout
        latest_layout = auto.wait_for_focused_master(base_url, target["surfaceId"], timeout_seconds) or {}
        if not latest_layout:
            return failed("focus-order", f"{action_name} did not promote target to master", surfaceId=target["surfaceId"]), events, {}
        layout_surface = auto.layout_surface_for(latest_layout, target["surfaceId"])
        events.append(
            {
                "action": action_name,
                "surfaceId": target["surfaceId"],
                "zoneId": layout_surface.get("zoneId", ""),
                "order": layout_surface.get("order", 0),
                "revision": latest_layout.get("revision", 0),
            }
        )
    return passed("focus-order", "focus next and previous update targetable layout focus", events=events), events, latest_layout


def exercise_promote_to_master(auto, structured, base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, list[dict], dict]:
    layout = auto.wait_for_auto_layout(base_url, launched, timeout_seconds) or {}
    ordered = ordered_layout_surfaces(auto, layout, launched)
    candidates = [surface for surface in ordered if surface.get("zoneId") not in ("master", "primary")]
    if not candidates and len(ordered) > 1:
        candidates = [ordered[-1]]
    if not candidates:
        return failed("focus-order", "promote-to-master requires a non-master target"), [], layout
    attempts = []
    latest_layout = layout
    for target in candidates:
        response = structured.post_json(base_url + "/api/layout/action", {"surfaceId": target["surfaceId"], "action": "promote"})
        attempt = {
            "action": "promoteToMaster",
            "surfaceId": target["surfaceId"],
            "fromZoneId": target.get("zoneId", ""),
            "response": response,
        }
        if response.get("status") != "accepted":
            attempts.append({**attempt, "result": "promote_rejected"})
            continue
        latest_layout = auto.wait_for_focused_master(base_url, target["surfaceId"], timeout_seconds) or latest_layout
        surface = auto.layout_surface_for(latest_layout, target["surfaceId"])
        if surface.get("focused") and surface.get("zoneId") in ("master", "primary"):
            event = {
                **attempt,
                "result": "promoted",
                "zoneId": surface.get("zoneId", ""),
                "order": surface.get("order", 0),
                "revision": latest_layout.get("revision", 0),
            }
            return passed("focus-order", "promote-to-master focuses and promotes a stack surface", event=event), [event], latest_layout
        attempts.append(
            {
                **attempt,
                "result": "not_promoted",
                "observedZoneId": surface.get("zoneId", ""),
                "observedFocused": surface.get("focused", False),
                "revision": latest_layout.get("revision", 0) if isinstance(latest_layout, dict) else 0,
            }
        )
    return failed("focus-order", "promote-to-master did not promote any stack candidate", attempts=attempts), attempts, latest_layout


def exercise_move_zone_recovery(auto, base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, list[dict], dict]:
    if len(launched) < 2:
        return failed("shell-action", "move-zone requires at least two active surfaces"), [], {}
    target = launched[-1]
    status, payload = post_json_result(
        base_url + "/api/layout/action",
        {
            "action": "assignZone",
            "surfaceId": target["surfaceId"],
            "workspaceId": "workspace-1",
            "zoneId": "secondary",
        },
    )
    events = [{"action": "moveZone", "surfaceId": target["surfaceId"], "zoneId": "secondary", "status": status, "response": payload}]
    if status >= 300:
        return failed("shell-action", "move-zone action failed", events=events), events, {}

    reset_status, reset_payload = post_json_result(base_url + "/api/layout/action", {"action": "setMode", "mode": "zones"})
    events.append({"action": "recoverAutoLayout", "mode": "zones", "status": reset_status, "response": reset_payload})
    if reset_status >= 300:
        return failed("shell-action", "auto-layout recovery after move-zone failed", events=events), events, {}
    layout_check, layout = wait_for_daily_layout(auto, base_url, launched, timeout_seconds)
    if layout_check["status"] != "pass":
        return failed("shell-action", "auto-layout did not recover after move-zone", events=events, layoutCheck=layout_check), events, layout
    return passed("shell-action", "move-zone control is accepted and recovers to auto-tiling", events=events), events, layout


def exercise_float_tile(auto, base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, list[dict], dict]:
    if not launched:
        return failed("shell-action", "float/tile requires an active surface"), [], {}
    target = launched[0]
    events = []
    status, payload = post_json_result(base_url + "/api/layout/action", {"action": "setFloating", "surfaceId": target["surfaceId"], "floating": True})
    events.append({"action": "setFloating", "floating": True, "surfaceId": target["surfaceId"], "status": status, "response": payload})
    if status >= 300:
        return failed("shell-action", "float action failed", events=events), events, {}
    layout = auto.wait_for_participation(base_url, target["surfaceId"], "floating", timeout_seconds) or {}
    if not layout:
        return failed("shell-action", "float action did not update layout participation", events=events), events, {}

    status, payload = post_json_result(base_url + "/api/layout/action", {"action": "setFloating", "surfaceId": target["surfaceId"], "floating": False})
    events.append({"action": "setFloating", "floating": False, "surfaceId": target["surfaceId"], "status": status, "response": payload})
    if status >= 300:
        return failed("shell-action", "tile action failed", events=events), events, layout
    retiled = auto.wait_for_participation(base_url, target["surfaceId"], "tiled", timeout_seconds) or {}
    if not retiled:
        return failed("shell-action", "tile action did not restore layout participation", events=events), events, {}
    return passed("shell-action", "float and tile controls update layout participation", events=events), events, retiled


def exercise_launcher_and_status(structured, base_url: str, timeout_seconds: float) -> tuple[dict, list[dict]]:
    events = []
    failures = []
    for app_id, expected_app_id in [("shell-launcher", "io.agorade.ShellLauncher"), ("shell-status", "io.agorade.ShellStatus")]:
        launch = structured.post_json(base_url + "/api/catalog/launch", {"appId": app_id})
        surface_id = launch.get("surfaceId") or ""
        events.append({"action": "launchPanel", "appId": app_id, "surfaceId": surface_id, "response": launch})
        if launch.get("status") != "launched" or not surface_id:
            failures.append({"appId": app_id, "reason": "launch_failed", "response": launch})
            continue
        surface = structured.wait_for_surface(base_url, surface_id, expected_app_id, timeout_seconds)
        if not surface:
            failures.append({"appId": app_id, "surfaceId": surface_id, "reason": "not_visible"})
            continue
        events.append({"action": "visiblePanel", "appId": app_id, "surfaceId": surface_id, "expectedAppId": expected_app_id})
        close = post_surface_action_with_retry(base_url, surface_id, "close", timeout_seconds)
        events.append({"action": "closePanel", "appId": app_id, "surfaceId": surface_id, "response": close})
        if close.get("status") != "accepted" or not structured.wait_until_absent(base_url, surface_id, timeout_seconds):
            failures.append({"appId": app_id, "surfaceId": surface_id, "reason": "close_failed", "response": close})
    if failures:
        return failed("launcher-status", "launcher/status panel workflow failed", failures=failures, events=events), events
    return passed("launcher-status", "launcher and status panel launch, display, and close", events=events), events


def exercise_close_relaunch_recovery(auto, structured, base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, list[dict], dict]:
    active = active_launched(launched)
    if not active:
        return failed("restart", "no active launched surface is available for restart check"), [], {}
    target = active[0]
    events = []
    close = post_surface_action_with_retry(base_url, target["surfaceId"], "close", timeout_seconds)
    if close.get("status") != "accepted":
        return failed("restart", "close before relaunch was not accepted", response=close), events, {}
    if not structured.wait_until_absent(base_url, target["surfaceId"], timeout_seconds):
        return failed("restart", "closed surface remained before relaunch", surfaceId=target["surfaceId"]), events, {}
    target["closed"] = True
    events.append({"action": "close", "surfaceId": target["surfaceId"]})

    launch_check, replacement = auto.launch_app(structured, base_url, target["appId"], target["expectedAppId"])
    if launch_check["status"] != "pass" or replacement is None:
        return failed("restart", "relaunch failed", launchCheck=launch_check), events, {}
    launched.append(replacement)
    surface = structured.wait_for_surface(base_url, replacement["surfaceId"], replacement["expectedAppId"], timeout_seconds)
    if not surface:
        return failed("restart", "replacement surface did not become visible", surfaceId=replacement["surfaceId"]), events, {}
    replacement["surface"] = surface
    events.append({"action": "relaunch", "surfaceId": replacement["surfaceId"], "appId": replacement["appId"]})

    layout_check, layout = wait_for_daily_layout(auto, base_url, active_launched(launched), timeout_seconds)
    if layout_check["status"] != "pass":
        return failed("restart", "auto-layout did not recover after relaunch", layoutCheck=layout_check, surfaceIds=auto.surface_ids(active_launched(launched))), events, layout
    return passed("restart", "close and relaunch recovered auto-layout without stale windows", events=events), events, layout


def post_surface_action_with_retry(base_url: str, surface_id: str, action: str, timeout_seconds: float) -> dict:
    deadline = time.time() + timeout_seconds
    latest = {}
    while time.time() < deadline:
        status, payload = post_json_result(base_url + "/api/surfaces/action", {"surfaceId": surface_id, "action": action})
        latest = {"httpStatus": status, **payload}
        if 200 <= status < 300:
            return payload
        if payload.get("errorClass") != "compositor_unavailable":
            return latest
        time.sleep(0.5)
    return latest


def check_daily_recovery(auto, layout: dict, launched: list[dict]) -> dict:
    if not layout:
        return failed("recovery", "no layout state is available after daily workflow")
    if (layout.get("settings") or {}).get("rule") != "master_stack":
        return failed("recovery", "daily workflow did not recover to master_stack rule", settings=layout.get("settings") or {})
    planner = auto.check_planner_state(layout, launched)
    placement = auto.check_backend_placement(layout, launched)
    occlusion = auto.check_occlusion(layout, launched)
    failures = [check for check in [planner, placement, occlusion] if check["status"] != "pass"]
    labels = {}
    for surface in auto.layout_surfaces_for(layout, launched):
        label = str(surface.get("label") or "").strip()
        if not label:
            failures.append(failed("recovery", "surface is missing targetable label", surfaceId=surface.get("surfaceId")))
        labels[surface.get("surfaceId")] = label
    if failures:
        return failed("recovery", "daily workflow did not recover to targetable non-overlapping auto-layout", failures=failures)
    return passed(
        "recovery",
        "daily workflow recovers to targetable non-overlapping auto-layout",
        revision=layout.get("revision", 0),
        labels=labels,
    )


def wait_for_daily_layout(auto, base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, dict]:
    deadline = time.time() + timeout_seconds
    latest_layout = {}
    latest_checks = []
    while time.time() < deadline:
        layout = auto.wait_for_auto_layout(base_url, launched, min(1.0, max(0.1, deadline - time.time()))) or {}
        if layout:
            latest_layout = layout
        planner = auto.check_planner_state(latest_layout, launched)
        placement = auto.check_backend_placement(latest_layout, launched)
        occlusion = auto.check_occlusion(latest_layout, launched)
        latest_checks = [planner, placement, occlusion]
        if all(check["status"] == "pass" for check in latest_checks):
            return (
                passed(
                    "daily-layout",
                    "daily workflow layout settled with planner, backend placement, and occlusion checks passing",
                    revision=latest_layout.get("revision", 0),
                ),
                latest_layout,
            )
        time.sleep(0.25)
    return (
        failed(
            "daily-layout",
            "daily workflow layout did not settle without overlap",
            latestChecks=latest_checks,
            revision=latest_layout.get("revision", 0) if isinstance(latest_layout, dict) else 0,
        ),
        latest_layout,
    )


def ordered_layout_surfaces(auto, layout: dict, launched: list[dict]) -> list[dict]:
    surfaces = auto.layout_surfaces_for(layout, launched)
    return sorted(surfaces, key=lambda surface: int(surface.get("order") or 0))


def active_launched(launched: list[dict]) -> list[dict]:
    return [item for item in launched if not item.get("closed")]


def get_text(url: str) -> str:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-daily-wm-workflow/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return response.read().decode("utf-8")


def post_json_result(url: str, body: dict) -> tuple[int, dict]:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-daily-wm-workflow/1"},
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return response.status, json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        try:
            payload = json.loads(detail)
        except json.JSONDecodeError:
            payload = {"error": detail}
        return error.code, payload


def classify_shell_action(name: str, detail: str, result: tuple[int, dict], **extra: object) -> dict:
    status, payload = result
    if 200 <= status < 300:
        return passed(name, detail, response=payload, **extra)
    return failed(name, "shell action failed", status=status, response=payload, **extra)


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "fail", "detail": detail, **extra}


def finish(
    checks: list[dict],
    evidence_packets: list[dict],
    app_ids: list[str],
    expected_app_ids: list[str],
    launched: list[dict],
    workflow_events: list[dict],
    layout: dict,
    checked_at: int,
) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": SCHEMA,
        "checkedAtUnixMillis": checked_at,
        "appIds": app_ids,
        "expectedAppIds": expected_app_ids,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "launched": launched,
        "workflowEvents": workflow_events,
        "layout": {
            "mode": layout.get("mode", "") if isinstance(layout, dict) else "",
            "revision": layout.get("revision", 0) if isinstance(layout, dict) else 0,
            "settings": layout.get("settings", {}) if isinstance(layout, dict) else {},
            "surfaces": layout.get("surfaces", []) if isinstance(layout, dict) else [],
            "workspaces": layout.get("workspaces", []) if isinstance(layout, dict) else [],
        },
        "summary": {
            "status": "fail" if failed_checks else "pass",
            "passed": len(checks) - len(failed_checks),
            "failed": len(failed_checks),
        },
    }
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 1 if failed_checks else 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import sys
import time
import urllib.error
import urllib.request


SCHEMA = "agora-de.auto-tiling-wm-live.v1"
SCENARIO = "den-k8-auto-tiling-wm-visible"
OLD_COMPOSITORCTL = pathlib.Path("/usr/local/bin/compositorctl")
SUCCESSFUL_LAUNCH_STATUSES = {"launched", "surface_observed_after_timeout", "reused_existing_window"}


def main() -> int:
    parser = argparse.ArgumentParser(description="Check installed-service auto-tiling WM behavior.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--overlay-app-id", default="io.agorade.ShellOverlay")
    parser.add_argument("--app-id", action="append", default=[], help="App id to launch; repeat for three or more apps.")
    parser.add_argument("--expected-app-id", action="append", default=[], help="Expected compositor app id matching each --app-id.")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-auto-tiling-wm")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.5)
    parser.add_argument("--require-capture", action="store_true")
    parser.add_argument("--timeout-seconds", type=float, default=12)
    args = parser.parse_args()

    structured = load_module("check-structured-layout.py", "agora_de_check_structured_layout")
    commands = load_module("check-layout-commands.py", "agora_de_check_layout_commands")
    overlay = load_module("check-overlay-labels.py", "agora_de_check_overlay_labels")
    app_ids = args.app_id or ["Alacritty.desktop", "foot.desktop", "firefox.desktop"]
    expected_app_ids = args.expected_app_id or ["Alacritty", "foot", "firefox"]
    checked_at = structured.unix_millis()
    checks = []
    evidence_packets = []
    launched = []
    focus_order = []
    restart_events = []
    escape_hatches = []
    latest_layout = {}

    if len(app_ids) < 3:
        checks.append(failed("config", "auto-tiling WM proof requires at least three --app-id values"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)
    if len(expected_app_ids) != len(app_ids):
        checks.append(failed("config", "--expected-app-id count must match --app-id count"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

    path_check = structured.check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

    try:
        shell_route = check_shell_controls_route(args.base_url)
        checks.append(shell_route)
        if shell_route["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        catalog_check = check_catalog(structured, args.base_url, app_ids)
        checks.append(catalog_check)
        if catalog_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        for app_id, expected_app_id in zip(app_ids, expected_app_ids):
            launch_check, item = launch_app(structured, args.base_url, app_id, expected_app_id)
            checks.append(launch_check)
            if launch_check["status"] != "pass" or item is None:
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)
            launched.append(item)

        visibility = wait_for_launched_surfaces(structured, args.base_url, active_launched(launched), args.timeout_seconds)
        checks.append(visibility)
        if visibility["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        set_mode = post_json_result(args.base_url + "/api/layout/action", {"action": "setMode", "mode": "zones"})
        checks.append(classify_shell_action("shell-action", "shell layout mode control accepted", set_mode, action="setMode"))
        if checks[-1]["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        latest_layout = wait_for_auto_layout(args.base_url, active_launched(launched), args.timeout_seconds) or {}
        planner_check = check_planner_state(latest_layout, active_launched(launched))
        checks.append(planner_check)
        if planner_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)
        placement = check_backend_placement(latest_layout, active_launched(launched))
        checks.append(placement)
        if placement["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)
        occlusion = check_occlusion(latest_layout, active_launched(launched))
        checks.append(occlusion)
        if occlusion["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        focus_check, focus_order, latest_layout = exercise_focus_order(structured, args.base_url, active_launched(launched), args.timeout_seconds)
        checks.append(focus_check)
        if focus_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        shell_escape, escape_hatches, latest_layout = exercise_shell_escape_hatches(args.base_url, active_launched(launched), args.timeout_seconds)
        checks.append(shell_escape)
        if shell_escape["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        agent_action, latest_layout = exercise_agent_controls(commands, args.compositorctl, active_launched(launched), args.timeout_seconds)
        checks.append(agent_action)
        if agent_action["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        restart_check, restart_events, latest_layout = exercise_close_relaunch(structured, args.base_url, launched, args.timeout_seconds)
        checks.append(restart_check)
        if restart_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        overlay_check = check_overlay_state(overlay, args.base_url, args.overlay_app_id, active_launched(launched), args.timeout_seconds)
        checks.append(overlay_check)
        if overlay_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        if args.output_name:
            if args.capture_delay_seconds > 0:
                time.sleep(args.capture_delay_seconds)
            capture_check, packet = structured.capture_visible_structured_layout(
                args.compositorctl,
                args.output_name,
                args.output_capture_session,
                checked_at,
                surface_ids(active_launched(launched)),
            )
            capture_check = dict(capture_check)
            if capture_check.get("status") == "pass":
                capture_check["name"] = "capture"
                capture_check["detail"] = "physical output capture shows deployed auto-tiling WM scenario"
            checks.append(capture_check)
            if packet:
                packet = dict(packet)
                packet["scenario"] = SCENARIO
                packet["surfaceIds"] = surface_ids(active_launched(launched))
                evidence_packets.append(packet)
        elif args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)

        cleanup_checks = cleanup_launched(structured, args.base_url, active_launched(launched), args.timeout_seconds)
        checks.extend(cleanup_checks)
    finally:
        for item in active_launched(launched):
            try:
                structured.post_json(args.base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
            except Exception:
                pass

    return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_order, restart_events, escape_hatches, latest_layout, checked_at)


def load_module(filename: str, module_name: str):
    path = pathlib.Path(__file__).with_name(filename)
    spec = importlib.util.spec_from_file_location(module_name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def check_shell_controls_route(base_url: str) -> dict:
    body = get_text(base_url + "/shell/dist/desktop/?surface=dock")
    required = [
        'id="wm-controls"',
        'id="focus-prev-button"',
        'id="focus-next-button"',
        'id="promote-button"',
        'id="move-zone-button"',
        'id="float-button"',
        'id="fullscreen-button"',
        'id="maximize-button"',
        'id="minimize-button"',
        'id="close-focus-button"',
        'id="reset-layout-button"',
        'id="layout-rule-label"',
        "/api/layout/action",
        "/api/surfaces/action",
    ]
    missing = [value for value in required if value not in body]
    if missing:
        return failed("shell-action", "dock route is missing WM control hooks", missing=missing)
    return passed("shell-action", "dock route exposes WM control hooks")


def check_catalog(structured, base_url: str, app_ids: list[str]) -> dict:
    catalog = structured.get_json(base_url + "/api/catalog/apps")
    apps = catalog.get("apps", [])
    failures = []
    for app_id in app_ids:
        app = next((item for item in apps if isinstance(item, dict) and item.get("id") == app_id), None)
        if not app:
            failures.append({"appId": app_id, "reason": "missing"})
        elif app.get("launchable") is not True:
            failures.append(
                {
                    "appId": app_id,
                    "reason": "not_launchable",
                    "disabledCode": app.get("disabledCode") or "",
                    "disabledReason": app.get("disabledReason") or "",
                }
            )
    if failures:
        return failed("launch", "one or more auto-tiling apps are not launchable", failures=failures)
    return passed("launch", "all auto-tiling apps are launchable", appIds=app_ids)


def launch_app(structured, base_url: str, app_id: str, expected_app_id: str) -> tuple[dict, dict | None]:
    launch = structured.post_json(base_url + "/api/catalog/launch", {"appId": app_id})
    surface_id = launch.get("surfaceId") or ""
    if launch.get("status") not in SUCCESSFUL_LAUNCH_STATUSES or not surface_id:
        return failed("launch", f"unexpected launch response for {app_id!r}", appId=app_id, response=launch), None
    return (
        passed("launch", "native app launched through shell catalog path", appId=app_id, expectedAppId=expected_app_id, surfaceId=surface_id),
        {"appId": app_id, "expectedAppId": expected_app_id, "surfaceId": surface_id, "closed": False},
    )


def wait_for_launched_surfaces(structured, base_url: str, launched: list[dict], timeout_seconds: float) -> dict:
    missing = []
    for item in launched:
        surface = structured.wait_for_surface(base_url, item["surfaceId"], item["expectedAppId"], timeout_seconds)
        if surface:
            item["surface"] = surface
        else:
            missing.append({"surfaceId": item["surfaceId"], "expectedAppId": item["expectedAppId"]})
    if missing:
        return failed("visibility", "one or more launched surfaces did not become mapped and visible", missing=missing)
    return passed("visibility", "all launched work surfaces are mapped and visible", surfaceIds=surface_ids(launched))


def wait_for_auto_layout(base_url: str, launched: list[dict], timeout_seconds: float) -> dict | None:
    wanted = set(surface_ids(launched))
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        layout = get_json(base_url + "/api/layout").get("layout") or {}
        surfaces = {surface.get("surfaceId"): surface for surface in layout.get("surfaces", []) if isinstance(surface, dict)}
        if wanted.issubset(surfaces) and all(is_tiled_with_geometry(surfaces[surface_id]) for surface_id in wanted):
            return layout
        time.sleep(0.25)
    return None


def is_tiled_with_geometry(surface: dict) -> bool:
    geometry = surface.get("geometry")
    return (
        surface.get("visible") is not False
        and surface.get("participation") == "tiled"
        and isinstance(geometry, dict)
        and int(geometry.get("width") or 0) > 0
        and int(geometry.get("height") or 0) > 0
    )


def check_planner_state(layout: dict, launched: list[dict]) -> dict:
    if not layout:
        return failed("planner-mismatch", "layout route did not report auto-tiling state")
    settings = layout.get("settings") or {}
    if layout.get("mode") != "zones":
        return failed("planner-mismatch", "auto-layout mode is not zones", mode=layout.get("mode") or "")
    if settings.get("rule") != "master_stack":
        return failed("planner-mismatch", "auto-layout rule is not master_stack", settings=settings)
    surfaces = layout_surfaces_for(layout, launched)
    if len(surfaces) != len(launched):
        return failed("planner-mismatch", "layout state is missing launched surfaces", expectedSurfaceIds=surface_ids(launched))
    return passed(
        "planner-mismatch",
        "backend reports master_stack zones layout for launched work surfaces",
        revision=layout.get("revision", 0),
        settings=settings,
    )


def check_backend_placement(layout: dict, launched: list[dict]) -> dict:
    failures = []
    placements = []
    for surface in layout_surfaces_for(layout, launched):
        geometry = surface.get("geometry")
        if not is_tiled_with_geometry(surface):
            failures.append({"surfaceId": surface.get("surfaceId"), "surface": surface})
        else:
            placements.append(
                {
                    "surfaceId": surface.get("surfaceId"),
                    "zoneId": surface.get("zoneId") or "",
                    "order": surface.get("order", 0),
                    "geometry": geometry,
                }
            )
    if failures:
        return failed("backend-placement", "one or more tiled surfaces lack acknowledged backend geometry", failures=failures)
    return passed("backend-placement", "auto-layout produced acknowledged backend geometry", placements=placements)


def check_occlusion(layout: dict, launched: list[dict]) -> dict:
    rectangles = {}
    for surface in layout_surfaces_for(layout, launched):
        geometry = surface.get("geometry")
        if not isinstance(geometry, dict):
            return failed("occlusion", "surface lacks geometry for occlusion check", surfaceId=surface.get("surfaceId"))
        rectangles[surface.get("surfaceId")] = geometry
    overlaps = []
    ids = list(rectangles)
    for index, left_id in enumerate(ids):
        for right_id in ids[index + 1 :]:
            area = overlap_area(rectangles[left_id], rectangles[right_id])
            if area > 4:
                overlaps.append({"left": left_id, "right": right_id, "area": area})
    if overlaps:
        return failed("occlusion", "auto-tiled work surfaces overlap", overlaps=overlaps, geometry=rectangles)
    return passed("occlusion", "auto-tiled work surfaces do not overlap", geometry=rectangles)


def exercise_focus_order(structured, base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, list[dict], dict]:
    sequence = []
    latest_layout = {}
    for index, item in enumerate(launched, start=1):
        result = structured.post_json(base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "focus"})
        if result.get("status") != "accepted":
            return failed("focus-order", "shell focus action was not accepted", surfaceId=item["surfaceId"], response=result), sequence, latest_layout
        latest_layout = wait_for_focused_master(base_url, item["surfaceId"], timeout_seconds) or {}
        if not latest_layout:
            return failed("focus-order", "surface did not become the focused master layout target", surfaceId=item["surfaceId"]), sequence, {}
        layout_surface = layout_surface_for(latest_layout, item["surfaceId"]) if latest_layout else {}
        sequence.append(
            {
                "index": index,
                "surfaceId": item["surfaceId"],
                "zoneId": layout_surface.get("zoneId", ""),
                "order": layout_surface.get("order", 0),
            }
        )
    if not latest_layout:
        latest_layout = get_json(base_url + "/api/layout").get("layout") or {}
    if sequence and sequence[-1].get("zoneId") not in ("master", "primary"):
        return failed("focus-order", "focused surface was not promoted into a primary/master zone", sequence=sequence), sequence, latest_layout
    return passed("focus-order", "focus actions update order and promote the focused surface", focusOrder=sequence), sequence, latest_layout


def wait_for_focused_master(base_url: str, surface_id: str, timeout_seconds: float) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        layout = get_json(base_url + "/api/layout").get("layout") or {}
        surface = layout_surface_for(layout, surface_id)
        if surface and surface.get("focused") and surface.get("zoneId") in ("master", "primary"):
            return layout
        time.sleep(0.25)
    return None


def exercise_shell_escape_hatches(base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, list[dict], dict]:
    if not launched:
        return failed("shell-action", "no launched surface is available for escape-hatch checks"), [], {}
    target = launched[0]
    events = []
    status, payload = post_json_result(base_url + "/api/layout/action", {"action": "setFloating", "surfaceId": target["surfaceId"], "floating": True})
    if status >= 300:
        return failed("shell-action", "shell floating action failed", status=status, payload=payload), events, {}
    events.append({"action": "setFloating", "floating": True, "response": payload})
    layout = wait_for_participation(base_url, target["surfaceId"], "floating", timeout_seconds) or {}
    if not layout:
        return failed("shell-action", "floating action did not update layout participation", surfaceId=target["surfaceId"]), events, {}

    status, payload = post_json_result(base_url + "/api/layout/action", {"action": "setFloating", "surfaceId": target["surfaceId"], "floating": False})
    if status >= 300:
        return failed("shell-action", "shell tile action failed", status=status, payload=payload), events, layout
    events.append({"action": "setFloating", "floating": False, "response": payload})
    retiled_layout = wait_for_participation(base_url, target["surfaceId"], "tiled", timeout_seconds)
    if not retiled_layout:
        return failed("shell-action", "tile action did not restore layout participation", surfaceId=target["surfaceId"]), events, {}
    layout = retiled_layout

    for action in ["fullscreen", "maximize", "minimize"]:
        for enabled in [True, False]:
            status, payload = post_json_result(
                base_url + "/api/surfaces/action",
                {"action": action, "surfaceId": target["surfaceId"], "enabled": enabled},
            )
            if status < 300 or payload.get("errorClass") == "backend_unsupported":
                events.append({"action": action, "enabled": enabled, "status": status, "response": payload})
                continue
            return failed(
                "shell-action",
                action + " action failed without backend classification",
                enabled=enabled,
                status=status,
                payload=payload,
            ), events, layout
    return passed("shell-action", "shell controls exercised floating, tiling, and compositor state actions", events=events), events, layout


def wait_for_participation(base_url: str, surface_id: str, participation: str, timeout_seconds: float) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        layout = get_json(base_url + "/api/layout").get("layout") or {}
        surface = layout_surface_for(layout, surface_id)
        if surface and surface.get("participation") == participation:
            return layout
        time.sleep(0.25)
    return None


def exercise_agent_controls(commands, compositorctl: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, dict]:
    if len(launched) < 2:
        return failed("agent-action", "agent controls require at least two active surfaces"), {}
    target = launched[-1]
    focus = commands.run_compositorctl_json(compositorctl, ["surface", "focus", "--surface", target["surfaceId"], "--timeout-ms", "2000"])
    if focus.get("decision") != "accepted":
        return failed("agent-action", "compositorctl focus was not accepted", response=focus), {}
    floating = commands.run_compositorctl_json(
        compositorctl,
        ["surface", "set-floating", "--surface", target["surfaceId"], "--enabled=false", "--timeout-ms", "2000"],
    )
    if floating.get("decision") != "accepted":
        return failed("agent-action", "compositorctl set-floating false was not accepted", response=floating), {}
    deadline = time.time() + timeout_seconds
    latest_layout = {}
    while time.time() < deadline:
        latest_layout = commands.normalize_cli_layout(commands.run_compositorctl_json(compositorctl, ["layout", "get"]).get("layout") or {})
        surface = layout_surface_for(latest_layout, target["surfaceId"])
        if surface and surface.get("focused") and surface.get("participation") == "tiled":
            return passed("agent-action", "compositorctl can target and retile a focused surface", surfaceId=target["surfaceId"]), latest_layout
        time.sleep(0.25)
    return failed("agent-action", "compositorctl action result was not visible in layout get", surfaceId=target["surfaceId"], layout=latest_layout), latest_layout


def exercise_close_relaunch(structured, base_url: str, launched: list[dict], timeout_seconds: float) -> tuple[dict, list[dict], dict]:
    active = active_launched(launched)
    if not active:
        return failed("restart", "no active launched surface is available for restart check"), [], {}
    target = active[0]
    events = []
    close = structured.post_json(base_url + "/api/surfaces/action", {"surfaceId": target["surfaceId"], "action": "close"})
    if close.get("status") != "accepted":
        return failed("restart", "close before relaunch was not accepted", response=close), events, {}
    if not structured.wait_until_absent(base_url, target["surfaceId"], timeout_seconds):
        return failed("restart", "closed surface remained before relaunch", surfaceId=target["surfaceId"]), events, {}
    target["closed"] = True
    events.append({"action": "close", "surfaceId": target["surfaceId"]})

    launch_check, replacement = launch_app(structured, base_url, target["appId"], target["expectedAppId"])
    if launch_check["status"] != "pass" or replacement is None:
        return failed("restart", "relaunch failed", launchCheck=launch_check), events, {}
    launched.append(replacement)
    surface = structured.wait_for_surface(base_url, replacement["surfaceId"], replacement["expectedAppId"], timeout_seconds)
    if not surface:
        return failed("restart", "replacement surface did not become visible", surfaceId=replacement["surfaceId"]), events, {}
    replacement["surface"] = surface
    events.append({"action": "relaunch", "surfaceId": replacement["surfaceId"], "appId": replacement["appId"]})
    layout = wait_for_auto_layout(base_url, active_launched(launched), timeout_seconds) or {}
    if not layout:
        return failed("restart", "auto-layout did not recover after relaunch", surfaceIds=surface_ids(active_launched(launched))), events, {}
    occlusion = check_occlusion(layout, active_launched(launched))
    if occlusion["status"] != "pass":
        return failed("restart", "relaunch left auto-tiled surfaces overlapping", occlusion=occlusion), events, layout
    return passed("restart", "close and relaunch recovered auto-layout without stale windows", events=events), events, layout


def check_overlay_state(overlay, base_url: str, overlay_app_id: str, launched: list[dict], timeout_seconds: float) -> dict:
    route = overlay.check_overlay_route(base_url)
    if route["status"] != "pass":
        return failed("overlay", "overlay route is not ready", route=route)
    overlay_surface = overlay.wait_for_overlay_surface(base_url, overlay_app_id, timeout_seconds)
    if not overlay_surface:
        return failed("overlay", "agent overlay surface is not mapped", overlayAppId=overlay_app_id)
    layout = overlay.wait_for_layout_surfaces(base_url, launched, timeout_seconds) or {}
    labels = overlay.check_layout_labels(layout, launched)
    if labels["status"] != "pass":
        return failed("overlay", "overlay label state does not match layout", labels=labels)
    return passed(
        "overlay",
        "agent overlay route and mapped surface expose layout labels",
        overlaySurfaceId=overlay_surface.get("id") or "",
        labels=labels.get("labels", {}),
    )


def cleanup_launched(structured, base_url: str, launched: list[dict], timeout_seconds: float) -> list[dict]:
    checks = []
    for item in launched:
        try:
            structured.post_json(base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
        except Exception as error:
            checks.append(failed("cleanup", "close action raised during cleanup", surfaceId=item["surfaceId"], error=str(error)))
            continue
        if structured.wait_until_absent(base_url, item["surfaceId"], timeout_seconds):
            item["closed"] = True
            checks.append(passed("cleanup", "surface closed and disappeared", surfaceId=item["surfaceId"]))
        else:
            checks.append(failed("cleanup", "surface remained after cleanup close", surfaceId=item["surfaceId"]))
    return checks


def get_text(url: str) -> str:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-auto-tiling-wm/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return response.read().decode("utf-8")


def get_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-auto-tiling-wm/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))


def post_json_result(url: str, body: dict) -> tuple[int, dict]:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-auto-tiling-wm/1"},
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


def layout_surfaces_for(layout: dict, launched: list[dict]) -> list[dict]:
    wanted = set(surface_ids(launched))
    return [surface for surface in layout.get("surfaces", []) if isinstance(surface, dict) and surface.get("surfaceId") in wanted]


def layout_surface_for(layout: dict, surface_id: str) -> dict:
    if not isinstance(layout, dict):
        return {}
    for surface in layout.get("surfaces", []):
        if isinstance(surface, dict) and surface.get("surfaceId") == surface_id:
            return surface
    return {}


def active_launched(launched: list[dict]) -> list[dict]:
    return [item for item in launched if not item.get("closed")]


def surface_ids(launched: list[dict]) -> list[str]:
    return [item["surfaceId"] for item in launched if item.get("surfaceId")]


def overlap_area(left: dict, right: dict) -> int:
    left_x2 = int(left.get("x", 0)) + int(left.get("width", 0))
    left_y2 = int(left.get("y", 0)) + int(left.get("height", 0))
    right_x2 = int(right.get("x", 0)) + int(right.get("width", 0))
    right_y2 = int(right.get("y", 0)) + int(right.get("height", 0))
    width = max(0, min(left_x2, right_x2) - max(int(left.get("x", 0)), int(right.get("x", 0))))
    height = max(0, min(left_y2, right_y2) - max(int(left.get("y", 0)), int(right.get("y", 0))))
    return width * height


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
    focus_order: list[dict],
    restart_events: list[dict],
    escape_hatches: list[dict],
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
        "focusOrder": focus_order,
        "restartEvents": restart_events,
        "escapeHatches": escape_hatches,
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

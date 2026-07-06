#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import subprocess
import sys
import time


OLD_COMPOSITORCTL = pathlib.Path("/usr/local/bin/compositorctl")
SUCCESSFUL_LAUNCH_STATUSES = {"launched", "surface_observed_after_timeout", "reused_existing_window"}


def main() -> int:
    parser = argparse.ArgumentParser(description="Check installed-service planner-backed layout behavior.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--app-id", action="append", default=[], help="App id to launch; repeat for three or more apps.")
    parser.add_argument("--expected-app-id", action="append", default=[], help="Expected compositor app id for each app.")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-planner-layout")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.5)
    parser.add_argument("--require-capture", action="store_true")
    parser.add_argument("--timeout-seconds", type=float, default=10)
    parser.add_argument("--planner-rule", choices=["master_stack"], default="master_stack")
    parser.add_argument("--nmaster", type=int, default=1)
    parser.add_argument("--mfact", type=float, default=0.6)
    parser.add_argument("--outer-horizontal-gap", type=int, default=0)
    parser.add_argument("--outer-vertical-gap", type=int, default=0)
    parser.add_argument("--inner-horizontal-gap", type=int, default=0)
    parser.add_argument("--inner-vertical-gap", type=int, default=0)
    parser.add_argument("--reserved-bottom-height", type=int, default=96)
    args = parser.parse_args()

    structured = load_module("check-structured-layout.py", "agora_de_check_structured_layout")
    commands = load_module("check-layout-commands.py", "agora_de_check_layout_commands")
    app_ids = args.app_id or ["Alacritty.desktop", "foot.desktop", "firefox.desktop"]
    expected_app_ids = args.expected_app_id or ["Alacritty", "foot", "firefox"]
    checked_at = structured.unix_millis()
    checks = []
    evidence_packets = []
    launched = []
    latest_layout = {}
    planner_summary = {}

    if len(app_ids) < 3:
        checks.append(failed("config", "planner layout proof requires at least three --app-id values"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
    if len(expected_app_ids) != len(app_ids):
        checks.append(failed("config", "expected app count must match app count"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)

    path_check = structured.check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)

    try:
        catalog = structured.get_json(args.base_url + "/api/catalog/apps")
        apps = catalog.get("apps", [])
        for app_id in app_ids:
            app = next((item for item in apps if isinstance(item, dict) and item.get("id") == app_id), None)
            if not app or app.get("launchable") is not True:
                checks.append(failed("catalog", f"app {app_id!r} is not launchable", appId=app_id))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
        checks.append(passed("catalog", "all planner-layout apps are launchable", appIds=app_ids))

        for app_id, expected_app_id in zip(app_ids, expected_app_ids):
            launch = structured.post_json(args.base_url + "/api/catalog/launch", {"appId": app_id})
            surface_id = launch.get("surfaceId") or ""
            if launch.get("status") not in SUCCESSFUL_LAUNCH_STATUSES or not surface_id:
                checks.append(failed("launch", f"unexpected launch response for {app_id!r}: {launch}", appId=app_id))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
            launched.append({"appId": app_id, "expectedAppId": expected_app_id, "surfaceId": surface_id})
            checks.append(passed("launch", "native app launched through shellui", appId=app_id, surfaceId=surface_id))

        for item in launched:
            surface = structured.wait_for_surface(args.base_url, item["surfaceId"], item["expectedAppId"], args.timeout_seconds)
            if not surface:
                checks.append(failed("visibility", f"surface {item['surfaceId']!r} did not become visible", surfaceId=item["surfaceId"]))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
            item["surface"] = surface
        checks.append(passed("visibility", "all launched planner surfaces are mapped and visible", surfaceIds=surface_ids(launched)))

        output = select_output(commands.run_compositorctl_json(args.compositorctl, ["output", "list"]), args.output_name)
        if not output:
            checks.append(failed("planner-mismatch", "no compositor output available for planner work area"))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
        plan = build_master_stack_plan(args, output, launched)
        planner_summary = {
            "rule": args.planner_rule,
            "settings": {
                "nmaster": args.nmaster,
                "mfact": args.mfact,
                "outerHorizontalGap": args.outer_horizontal_gap,
                "outerVerticalGap": args.outer_vertical_gap,
                "innerHorizontalGap": args.inner_horizontal_gap,
                "innerVerticalGap": args.inner_vertical_gap,
                "reservedBottomHeight": args.reserved_bottom_height,
            },
            "output": output,
            "expectedRectangles": plan,
        }
        checks.append(passed("planner-mismatch", "master-stack planner produced expected rectangles", planner=planner_summary))

        for item in launched:
            focus = commands.run_compositorctl_json(args.compositorctl, ["surface", "focus", "--surface", item["surfaceId"], "--timeout-ms", "2000"])
            if focus.get("decision") != "accepted":
                checks.append(failed("focus-order", "focus command was not accepted", surfaceId=item["surfaceId"], response=focus))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
        checks.append(passed("focus-order", "focus command accepted for each planned surface", focusOrder=surface_ids(launched)))

        for planned in plan:
            response = commands.run_compositorctl_json(
                args.compositorctl,
                [
                    "surface",
                    "assign-zone",
                    "--surface",
                    planned["surfaceId"],
                    "--workspace",
                    "workspace-1",
                    "--zone",
                    planned["zoneId"],
                    "--x",
                    str(planned["geometry"]["x"]),
                    "--y",
                    str(planned["geometry"]["y"]),
                    "--width",
                    str(planned["geometry"]["width"]),
                    "--height",
                    str(planned["geometry"]["height"]),
                    "--timeout-ms",
                    "2000",
                ],
            )
            if response.get("decision") != "accepted":
                checks.append(failed("backend-placement", "planner rectangle placement was not accepted", planned=planned, response=response))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
        checks.append(passed("backend-placement", "Wayfire adapter accepted planner rectangles", plannedSurfaceIds=surface_ids(launched)))

        latest_layout = wait_for_planned_layout(commands, args.compositorctl, plan, args.timeout_seconds) or {}
        if not latest_layout:
            latest_layout = commands.normalize_cli_layout(commands.run_compositorctl_json(args.compositorctl, ["layout", "get"]).get("layout") or {})
            checks.append(failed("backend-placement", "layout get did not acknowledge planner rectangles", expected=plan, layout=latest_layout))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
        checks.append(passed("backend-placement", "layout get acknowledged planner rectangles", revision=latest_layout.get("revision", 0)))

        overlap = check_no_overlap(latest_layout, plan)
        checks.append(overlap)
        if overlap["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)

        if args.output_name:
            if args.capture_delay_seconds > 0:
                time.sleep(args.capture_delay_seconds)
            capture_check, packet = structured.capture_visible_structured_layout(
                args.compositorctl,
                args.output_name,
                args.output_capture_session,
                checked_at,
                surface_ids(launched),
            )
            capture_check = dict(capture_check)
            capture_check["name"] = "capture"
            checks.append(capture_check)
            if packet:
                packet = dict(packet)
                packet["scenario"] = "den-k8-planner-layout-visible"
                evidence_packets.append(packet)
        elif args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)

        cleanup_checks = []
        for item in launched:
            close = commands.run_compositorctl_json(args.compositorctl, ["surface", "close", "--surface", item["surfaceId"]])
            if close.get("decision") != "accepted":
                cleanup_checks.append(failed("cleanup", "close command was not accepted", surfaceId=item["surfaceId"], response=close))
                continue
            if structured.wait_until_absent(args.base_url, item["surfaceId"], args.timeout_seconds):
                cleanup_checks.append(passed("cleanup", "surface closed and disappeared", surfaceId=item["surfaceId"]))
            else:
                cleanup_checks.append(failed("cleanup", "surface remained after close", surfaceId=item["surfaceId"]))
        checks.extend(cleanup_checks)
        if any(check["status"] != "pass" for check in cleanup_checks):
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)
    finally:
        for item in launched:
            try:
                structured.post_json(args.base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
            except Exception:
                pass

    return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, planner_summary, checked_at)


def load_module(filename: str, module_name: str):
    path = pathlib.Path(__file__).with_name(filename)
    spec = importlib.util.spec_from_file_location(module_name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def select_output(payload: dict, output_name: str) -> dict:
    outputs = payload.get("outputs", [])
    if not isinstance(outputs, list):
        return {}
    if output_name:
        for output in outputs:
            if isinstance(output, dict) and output.get("name") == output_name:
                return output
        return {}
    for output in outputs:
        if isinstance(output, dict):
            return output
    return {}


def build_master_stack_plan(args: argparse.Namespace, output: dict, launched: list[dict]) -> list[dict]:
    x = int(output.get("physical_x") or 0)
    y = int(output.get("physical_y") or 0)
    width = int(output.get("physical_width") or output.get("width") or 1)
    height = int(output.get("physical_height") or output.get("height") or 1)
    height = max(1, height - max(0, args.reserved_bottom_height))
    area = inset_rect(
        {"x": x, "y": y, "width": width, "height": height},
        args.outer_horizontal_gap,
        args.outer_vertical_gap,
    )
    nmaster = min(max(0, args.nmaster), len(launched))
    stack_count = max(0, len(launched) - nmaster)
    inner_h = clamp_gap(args.inner_horizontal_gap, area["width"]) if nmaster and stack_count else 0
    available_width = max(1, area["width"] - inner_h)
    mfact = min(0.9, max(0.1, args.mfact))
    if stack_count == 0:
        master_width = area["width"]
    elif nmaster == 0:
        master_width = 0
    else:
        master_width = min(available_width, round(available_width * mfact))
    stack_width = 0 if stack_count == 0 else max(1, available_width - master_width)
    master_area = {"x": area["x"], "y": area["y"], "width": max(1, master_width), "height": max(1, area["height"])}
    stack_area = {
        "x": area["x"] + master_width + inner_h,
        "y": area["y"],
        "width": stack_width,
        "height": max(1, area["height"]),
    }
    master_slices = vertical_slices(master_area, nmaster, args.inner_vertical_gap)
    stack_slices = vertical_slices(stack_area, stack_count, args.inner_vertical_gap)
    plan = []
    tiled_seen = 0
    for item in launched:
        if tiled_seen < nmaster:
            zone_id = "master"
            geometry = master_slices[tiled_seen]
        else:
            zone_id = "stack"
            geometry = stack_slices[tiled_seen - nmaster]
        plan.append({"surfaceId": item["surfaceId"], "zoneId": zone_id, "geometry": geometry})
        tiled_seen += 1
    return plan


def inset_rect(rect: dict, outer_h: int, outer_v: int) -> dict:
    hgap = min(max(0, outer_h), max(0, rect["width"] - 1) // 2)
    vgap = min(max(0, outer_v), max(0, rect["height"] - 1) // 2)
    return {
        "x": rect["x"] + hgap,
        "y": rect["y"] + vgap,
        "width": max(1, rect["width"] - hgap * 2),
        "height": max(1, rect["height"] - vgap * 2),
    }


def vertical_slices(area: dict, count: int, requested_gap: int) -> list[dict]:
    if count <= 0:
        return []
    if count == 1:
        return [area]
    gap = min(max(0, requested_gap), max(0, area["height"] - count) // (count - 1))
    total_gap = gap * (count - 1)
    available = max(count, area["height"] - total_gap)
    base = available // count
    remainder = available % count
    y = area["y"]
    slices = []
    for _ in range(count):
        extra = 1 if remainder > 0 else 0
        remainder -= extra
        height = max(1, base + extra)
        slices.append({"x": area["x"], "y": y, "width": max(1, area["width"]), "height": height})
        y += height + gap
    return slices


def clamp_gap(requested_gap: int, extent: int) -> int:
    return min(max(0, requested_gap), max(0, extent - 1))


def wait_for_planned_layout(commands, compositorctl: str, plan: list[dict], timeout_seconds: float) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        layout = commands.normalize_cli_layout(commands.run_compositorctl_json(compositorctl, ["layout", "get"]).get("layout") or {})
        surfaces = {surface.get("surfaceId"): surface for surface in layout.get("surfaces", [])}
        if all(matches_plan(surfaces.get(planned["surfaceId"]), planned) for planned in plan):
            return layout
        time.sleep(0.25)
    return None


def matches_plan(surface: dict | None, planned: dict) -> bool:
    if not surface or surface.get("zoneId") != planned["zoneId"]:
        return False
    geometry = surface.get("geometry") or {}
    expected = planned["geometry"]
    for key in ["x", "y", "width", "height"]:
        if abs(int(geometry.get(key, -999999)) - int(expected[key])) > 2:
            return False
    return True


def check_no_overlap(layout: dict, plan: list[dict]) -> dict:
    surfaces = {surface.get("surfaceId"): surface for surface in layout.get("surfaces", [])}
    rectangles = []
    for planned in plan:
        surface = surfaces.get(planned["surfaceId"]) or {}
        geometry = surface.get("geometry")
        if not isinstance(geometry, dict):
            return failed("backend-placement", "planned surface lacks backend geometry", surfaceId=planned["surfaceId"])
        rectangles.append((planned["surfaceId"], geometry))
    overlaps = []
    for index, (left_id, left) in enumerate(rectangles):
        for right_id, right in rectangles[index + 1 :]:
            if overlaps_materially(left, right):
                overlaps.append({"left": left_id, "right": right_id})
    if overlaps:
        return failed("backend-placement", "planner-backed surfaces overlap", overlaps=overlaps)
    return passed("backend-placement", "planner-backed surfaces do not overlap", surfaceIds=[item[0] for item in rectangles])


def overlaps_materially(left: dict, right: dict) -> bool:
    lx2 = int(left.get("x", 0)) + int(left.get("width", 0))
    ly2 = int(left.get("y", 0)) + int(left.get("height", 0))
    rx2 = int(right.get("x", 0)) + int(right.get("width", 0))
    ry2 = int(right.get("y", 0)) + int(right.get("height", 0))
    overlap_w = max(0, min(lx2, rx2) - max(int(left.get("x", 0)), int(right.get("x", 0))))
    overlap_h = max(0, min(ly2, ry2) - max(int(left.get("y", 0)), int(right.get("y", 0))))
    return overlap_w * overlap_h > 4


def surface_ids(launched: list[dict]) -> list[str]:
    return [item["surfaceId"] for item in launched if item.get("surfaceId")]


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
    layout: dict,
    planner: dict,
    checked_at: int,
) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": "agora-de.planner-layout-live.v1",
        "checkedAtUnixMillis": checked_at,
        "appIds": app_ids,
        "expectedAppIds": expected_app_ids,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "launched": launched,
        "planner": planner,
        "layout": layout,
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

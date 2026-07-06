#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import sys
import time
import urllib.error
import urllib.request


OLD_COMPOSITORCTL = pathlib.Path("/usr/local/bin/compositorctl")
SUCCESSFUL_LAUNCH_STATUSES = {"launched", "surface_observed_after_timeout", "reused_existing_window"}


def main() -> int:
    parser = argparse.ArgumentParser(description="Check installed-service structured layout behavior.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--app-id", action="append", default=[], help="App id to launch; repeat for two or more apps.")
    parser.add_argument(
        "--expected-app-id",
        action="append",
        default=[],
        help="Expected compositor app id matching each --app-id.",
    )
    parser.add_argument(
        "--expected-zone",
        action="append",
        default=[],
        help="Expected structured zone matching each --app-id.",
    )
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-structured-layout")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.5)
    parser.add_argument(
        "--require-capture",
        action="store_true",
        help="Fail unless --output-name supplies visible physical-output capture evidence.",
    )
    parser.add_argument("--timeout-seconds", type=float, default=10)
    args = parser.parse_args()

    app_ids = args.app_id or ["Alacritty.desktop", "foot.desktop"]
    expected_app_ids = args.expected_app_id or ["Alacritty", "foot"]
    expected_zones = args.expected_zone or ["primary", "secondary"]
    checked_at = unix_millis()
    checks = []
    evidence_packets = []
    launched = []
    latest_layout = {}

    if len(app_ids) < 2:
        checks.append(failed("config", "structured layout requires at least two --app-id values"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
    if len(expected_app_ids) != len(app_ids):
        checks.append(failed("config", "--expected-app-id count must match --app-id count"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
    if len(expected_zones) != len(app_ids):
        checks.append(failed("config", "--expected-zone count must match --app-id count"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

    path_check = check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

    try:
        catalog = get_json(args.base_url + "/api/catalog/apps")
        apps = catalog.get("apps", [])
        if not isinstance(apps, list):
            checks.append(failed("catalog", "catalog route response must contain apps array"))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

        for app_id in app_ids:
            app = next((item for item in apps if isinstance(item, dict) and item.get("id") == app_id), None)
            if not app:
                checks.append(failed("catalog", f"app {app_id!r} not present"))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
            if app.get("launchable") is not True:
                checks.append(
                    failed(
                        "catalog",
                        f"app {app_id!r} is not launchable",
                        appId=app_id,
                        disabledCode=app.get("disabledCode") or "",
                        disabledReason=app.get("disabledReason") or "",
                    )
                )
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
        checks.append(passed("catalog", "all structured-layout apps are launchable", appIds=app_ids))

        for app_id, expected_app_id in zip(app_ids, expected_app_ids):
            launch = post_json(args.base_url + "/api/catalog/launch", {"appId": app_id})
            surface_id = launch.get("surfaceId") or ""
            if launch.get("status") not in SUCCESSFUL_LAUNCH_STATUSES or not surface_id:
                checks.append(failed("launch", f"unexpected launch response for {app_id!r}: {launch}", appId=app_id))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
            launched.append({"appId": app_id, "expectedAppId": expected_app_id, "surfaceId": surface_id})
            checks.append(
                passed(
                    "launch",
                    "native app launched through installed shell path",
                    appId=app_id,
                    expectedAppId=expected_app_id,
                    surfaceId=surface_id,
                    launchId=launch.get("launchId") or "",
                )
            )

        for item in launched:
            surface = wait_for_surface(args.base_url, item["surfaceId"], item["expectedAppId"], args.timeout_seconds)
            if not surface:
                checks.append(
                    failed(
                        "visibility",
                        f"surface {item['surfaceId']!r} did not appear mapped/visible",
                        surfaceId=item["surfaceId"],
                        expectedAppId=item["expectedAppId"],
                    )
                )
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
            item["surface"] = surface
        checks.append(passed("visibility", "all launched surfaces are mapped and visible", surfaceIds=surface_ids(launched)))

        for item in launched:
            post_json(args.base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "focus"})
            focused = wait_for_surface(
                args.base_url,
                item["surfaceId"],
                item["expectedAppId"],
                args.timeout_seconds,
                focused=True,
            )
            if not focused:
                checks.append(failed("focus", f"surface {item['surfaceId']!r} did not become focused", surfaceId=item["surfaceId"]))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
        checks.append(passed("focus", "focus moved to each launched surface", surfaceIds=surface_ids(launched)))

        latest_layout = get_json(args.base_url + "/api/layout").get("layout") or {}
        layout_surfaces = layout_surfaces_for(latest_layout, launched)
        if len(layout_surfaces) != len(launched):
            checks.append(
                failed(
                    "layout-route",
                    "layout route did not include every launched surface",
                    expectedSurfaceIds=surface_ids(launched),
                    layoutSurfaceIds=[surface.get("surfaceId") for surface in layout_surfaces],
                )
            )
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
        checks.append(passed("layout-route", "layout route includes launched surfaces", mode=latest_layout.get("mode") or ""))

        assignment = assign_expected_zones(args.base_url, launched, expected_zones)
        checks.append(assignment)
        if assignment["status"] == "pass":
            latest_layout = wait_for_expected_zones(args.base_url, launched, expected_zones, args.timeout_seconds) or latest_layout
        else:
            latest_layout = get_json(args.base_url + "/api/layout").get("layout") or latest_layout

        occlusion = check_occlusion_or_zones(latest_layout, launched, expected_zones)
        checks.append(occlusion)
        if occlusion["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

        if args.output_name:
            if args.capture_delay_seconds > 0:
                time.sleep(args.capture_delay_seconds)
            capture_check, packet = capture_visible_structured_layout(
                args.compositorctl,
                args.output_name,
                args.output_capture_session,
                checked_at,
                surface_ids(launched),
            )
            checks.append(capture_check)
            if packet:
                evidence_packets.append(packet)
        elif args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))
            evidence_packets.append(unavailable_packet(checked_at, surface_ids(launched)))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

        cleanup_checks = cleanup_launched(args.base_url, launched, args.timeout_seconds)
        checks.extend(cleanup_checks)
    finally:
        for item in launched:
            try:
                post_json(args.base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
            except Exception:
                pass

    return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)


def check_compositorctl_path(compositorctl: str) -> dict:
    path = pathlib.Path(compositorctl)
    try:
        resolved = path.resolve(strict=False)
    except OSError:
        resolved = path
    if path == OLD_COMPOSITORCTL or resolved == OLD_COMPOSITORCTL:
        return failed("compositorctl-path", "old /usr/local/bin/compositorctl path is not allowed")
    if path.name != "agora-de-compositorctl":
        return failed("compositorctl-path", "structured layout evidence must use agora-de-compositorctl", path=str(path))
    return passed("compositorctl-path", "using agora-de compositorctl", path=str(path))


def get_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-structured-layout/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))


def post_json(url: str, body: dict) -> dict:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-structured-layout/1"},
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{url} returned {error.code}: {detail}") from error


def post_json_result(url: str, body: dict) -> tuple[int, dict]:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-structured-layout/1"},
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


def wait_for_surface(
    base_url: str,
    surface_id: str,
    expected_app_id: str,
    timeout_seconds: float,
    focused: bool = False,
) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
            visible = surface.get("visible")
            if visible is None:
                visible = surface.get("mapped")
            if (
                surface.get("id") == surface_id
                and surface.get("appId") == expected_app_id
                and surface.get("mapped")
                and visible
                and (not focused or surface.get("focused"))
            ):
                return surface
        time.sleep(0.25)
    return None


def wait_until_absent(base_url: str, surface_id: str, timeout_seconds: float) -> bool:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        surfaces = get_json(base_url + "/api/surfaces").get("surfaces", [])
        if not any(surface.get("id") == surface_id for surface in surfaces):
            return True
        time.sleep(0.25)
    return False


def assign_expected_zones(base_url: str, launched: list[dict], expected_zones: list[str]) -> dict:
    unsupported = []
    accepted = []
    failures = []
    for item, zone_id in zip(launched, expected_zones):
        status, payload = post_json_result(
            base_url + "/api/layout/action",
            {
                "action": "assignZone",
                "surfaceId": item["surfaceId"],
                "workspaceId": "workspace-1",
                "zoneId": zone_id,
            },
        )
        if 200 <= status < 300:
            accepted.append({"surfaceId": item["surfaceId"], "zoneId": zone_id})
        elif payload.get("errorClass") == "backend_unsupported":
            unsupported.append({"surfaceId": item["surfaceId"], "zoneId": zone_id, "error": payload.get("error") or ""})
        else:
            failures.append({"surfaceId": item["surfaceId"], "zoneId": zone_id, "status": status, "payload": payload})
    if failures:
        return failed("zone-assignment", "zone assignment failed with non-layout backend error", failures=failures)
    if unsupported:
        return passed(
            "zone-assignment",
            "layout backend does not yet support zone assignment; occlusion must pass by geometry or existing zones",
            unsupported=unsupported,
        )
    return passed("zone-assignment", "expected zone assignments accepted", assignments=accepted)


def wait_for_expected_zones(base_url: str, launched: list[dict], expected_zones: list[str], timeout_seconds: float) -> dict | None:
    expected = dict(zip(surface_ids(launched), expected_zones))
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        layout = get_json(base_url + "/api/layout").get("layout") or {}
        surfaces = layout_surfaces_for(layout, launched)
        if len(surfaces) == len(launched) and all(surface.get("zoneId") == expected.get(surface.get("surfaceId")) for surface in surfaces):
            return layout
        time.sleep(0.25)
    return None


def layout_surfaces_for(layout: dict, launched: list[dict]) -> list[dict]:
    wanted = set(surface_ids(launched))
    return [surface for surface in layout.get("surfaces", []) if isinstance(surface, dict) and surface.get("surfaceId") in wanted]


def check_occlusion_or_zones(layout: dict, launched: list[dict], expected_zones: list[str]) -> dict:
    surfaces = layout_surfaces_for(layout, launched)
    by_id = {surface.get("surfaceId"): surface for surface in surfaces}
    expected = dict(zip(surface_ids(launched), expected_zones))
    assigned_zones = {surface_id: (by_id.get(surface_id) or {}).get("zoneId") for surface_id in expected}
    distinct_expected_zones = all(assigned_zones.get(surface_id) == zone_id for surface_id, zone_id in expected.items()) and len(set(assigned_zones.values())) == len(assigned_zones)
    if distinct_expected_zones:
        return passed("occlusion-overlap", "surfaces are assigned to distinct expected zones", zones=assigned_zones)

    rectangles = {}
    missing_geometry = []
    for surface_id, surface in by_id.items():
        geometry = surface.get("geometry")
        if not isinstance(geometry, dict):
            missing_geometry.append(surface_id)
            continue
        rectangles[surface_id] = geometry

    if missing_geometry:
        return failed(
            "occlusion-overlap",
            "layout route lacks geometry and surfaces are not in distinct expected zones",
            missingGeometry=missing_geometry,
            zones=assigned_zones,
        )

    overlaps = []
    ids = list(rectangles)
    for left_index, left_id in enumerate(ids):
        for right_id in ids[left_index + 1 :]:
            area = overlap_area(rectangles[left_id], rectangles[right_id])
            if area > 0:
                overlaps.append({"left": left_id, "right": right_id, "area": area})
    if overlaps:
        return failed(
            "occlusion-overlap",
            "surfaces overlap and are not assigned to distinct expected zones",
            overlaps=overlaps,
            geometry=rectangles,
            zones=assigned_zones,
        )
    return passed("occlusion-overlap", "surface geometry is non-overlapping", geometry=rectangles, zones=assigned_zones)


def overlap_area(left: dict, right: dict) -> int:
    left_x2 = int(left.get("x", 0)) + int(left.get("width", 0))
    left_y2 = int(left.get("y", 0)) + int(left.get("height", 0))
    right_x2 = int(right.get("x", 0)) + int(right.get("width", 0))
    right_y2 = int(right.get("y", 0)) + int(right.get("height", 0))
    width = max(0, min(left_x2, right_x2) - max(int(left.get("x", 0)), int(right.get("x", 0))))
    height = max(0, min(left_y2, right_y2) - max(int(left.get("y", 0)), int(right.get("y", 0))))
    return width * height


def cleanup_launched(base_url: str, launched: list[dict], timeout_seconds: float) -> list[dict]:
    checks = []
    for item in launched:
        post_json(base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
        if wait_until_absent(base_url, item["surfaceId"], timeout_seconds):
            checks.append(passed("cleanup", "closed surface disappeared from running state", surfaceId=item["surfaceId"]))
        else:
            checks.append(failed("cleanup", "closed surface remained in running state", surfaceId=item["surfaceId"]))
    return checks


def capture_visible_structured_layout(
    compositorctl: str,
    output_name: str,
    session_id: str,
    checked_at: int,
    surface_ids_value: list[str],
) -> tuple[dict, dict | None]:
    live_evidence = load_live_evidence_module()
    capture_check, packet = live_evidence.capture_and_classify_output(
        compositorctl,
        output_name,
        session_id,
        checked_at,
    )
    if packet:
        packet = dict(packet)
        packet["scenario"] = "den-k8-structured-layout-visible"
        packet["surfaceIds"] = surface_ids_value
    if capture_check.get("status") == "pass":
        capture_check = dict(capture_check)
        capture_check["name"] = "structured-layout-visible-capture"
        capture_check["detail"] = "physical output capture shows structured layout scenario"
        capture_check["surfaceIds"] = surface_ids_value
    return capture_check, packet


def load_live_evidence_module():
    path = pathlib.Path(__file__).with_name("check-den-k8.py")
    spec = importlib.util.spec_from_file_location("agora_de_check_den_k8", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load live evidence module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def unavailable_packet(checked_at: int, surface_ids_value: list[str]) -> dict:
    return {
        "scenario": "den-k8-structured-layout-visible",
        "capturedAtUnixMillis": checked_at,
        "surfaceIds": surface_ids_value,
        "visualStatus": "unknown",
        "captureClassification": "not_visible",
    }


def surface_ids(launched: list[dict]) -> list[str]:
    return [item["surfaceId"] for item in launched if item.get("surfaceId")]


def unix_millis() -> int:
    return int(time.time() * 1000)


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
    checked_at: int,
) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": "agora-de.structured-layout-live.v1",
        "checkedAtUnixMillis": checked_at,
        "appIds": app_ids,
        "expectedAppIds": expected_app_ids,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "launched": launched,
        "layout": {
            "mode": layout.get("mode", "") if isinstance(layout, dict) else "",
            "revision": layout.get("revision", 0) if isinstance(layout, dict) else 0,
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

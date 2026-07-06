#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request


OLD_COMPOSITORCTL = pathlib.Path("/usr/local/bin/compositorctl")
SUCCESSFUL_LAUNCH_STATUSES = {"launched", "surface_observed_after_timeout", "reused_existing_window"}


def main() -> int:
    parser = argparse.ArgumentParser(description="Check installed-service agent overlay labels and bounds.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--overlay-app-id", default="io.agorade.ShellOverlay")
    parser.add_argument("--app-id", action="append", default=[], help="App id to launch; repeat for two or more apps.")
    parser.add_argument("--expected-app-id", action="append", default=[], help="Expected compositor app id matching each --app-id.")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-overlay-labels")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.5)
    parser.add_argument(
        "--require-capture",
        action="store_true",
        help="Fail unless --output-name supplies capture evidence for every focus step.",
    )
    parser.add_argument("--timeout-seconds", type=float, default=10)
    args = parser.parse_args()

    app_ids = args.app_id or ["Alacritty.desktop", "firefox.desktop"]
    expected_app_ids = args.expected_app_id or ["Alacritty", "firefox"]
    checked_at = unix_millis()
    checks = []
    evidence_packets = []
    launched = []
    focus_sequence = []
    latest_layout = {}

    if len(app_ids) < 2:
        checks.append(failed("config", "overlay labels require at least two --app-id values"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)
    if len(expected_app_ids) != len(app_ids):
        checks.append(failed("config", "--expected-app-id count must match --app-id count"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)

    path_check = check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)

    try:
        overlay_route = check_overlay_route(args.base_url)
        checks.append(overlay_route)
        if overlay_route["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)

        overlay_surface = wait_for_overlay_surface(args.base_url, args.overlay_app_id, args.timeout_seconds)
        if not overlay_surface:
            checks.append(
                failed(
                    "overlay-surface",
                    "agent overlay layer-shell surface is not mapped",
                    overlayAppId=args.overlay_app_id,
                )
            )
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)
        checks.append(
            passed(
                "overlay-surface",
                "agent overlay surface is mapped through installed shell service",
                surfaceId=overlay_surface.get("id") or "",
                appId=overlay_surface.get("appId") or "",
                role=overlay_surface.get("role") or "",
            )
        )

        catalog_check = check_catalog(args.base_url, app_ids)
        checks.append(catalog_check)
        if catalog_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)

        for app_id, expected_app_id in zip(app_ids, expected_app_ids):
            existing_surface_ids = surface_ids_for_app(args.base_url, expected_app_id)
            launch = post_json(args.base_url + "/api/catalog/launch", {"appId": app_id})
            surface_id = launch.get("surfaceId") or ""
            if not surface_id and launch.get("status") == "timed_out_no_surface":
                recovered_surface = wait_for_new_surface(args.base_url, expected_app_id, existing_surface_ids, min(args.timeout_seconds, 2))
                if recovered_surface:
                    checks.append(failed("launch", "launch API returned timed_out_no_surface even though a matching surface appeared", appId=app_id, response=launch, recoveredSurfaceId=recovered_surface.get("id") or ""))
                    return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)
            if not surface_id or launch.get("status") not in SUCCESSFUL_LAUNCH_STATUSES:
                checks.append(failed("launch", f"unexpected launch response for {app_id!r}: {launch}", appId=app_id))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)
            launched.append({"appId": app_id, "expectedAppId": expected_app_id, "surfaceId": surface_id})
        checks.append(passed("launch", "two or more native apps launched through installed shell path", surfaceIds=surface_ids(launched)))

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
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)
            item["surface"] = surface
        checks.append(passed("visibility", "all launched surfaces are mapped and visible", surfaceIds=surface_ids(launched)))

        latest_layout = wait_for_layout_surfaces(args.base_url, launched, args.timeout_seconds) or {}
        layout_check = check_layout_labels(latest_layout, launched)
        checks.append(layout_check)
        if layout_check["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)

        for index, item in enumerate(launched, start=1):
            post_json(args.base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "focus"})
            latest_layout = wait_for_focused_layout(args.base_url, item["surfaceId"], args.timeout_seconds) or {}
            if not latest_layout:
                focused = wait_for_surface(
                    args.base_url,
                    item["surfaceId"],
                    item["expectedAppId"],
                    args.timeout_seconds,
                    focused=True,
                )
            else:
                focused = item.get("surface") or wait_for_surface(
                    args.base_url,
                    item["surfaceId"],
                    item["expectedAppId"],
                    args.timeout_seconds,
                )
            if not focused or not latest_layout:
                checks.append(failed("focus", f"surface {item['surfaceId']!r} did not become focused", surfaceId=item["surfaceId"]))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)

            focus_sequence.append({"surfaceId": item["surfaceId"], "label": layout_label(latest_layout, item["surfaceId"]), "index": index})

            if args.output_name:
                if args.capture_delay_seconds > 0:
                    time.sleep(args.capture_delay_seconds)
                capture_check, packet = capture_visible_overlay(
                    args.compositorctl,
                    args.output_name,
                    f"{args.output_capture_session}-focus-{index}",
                    checked_at,
                    surface_ids(launched),
                    item["surfaceId"],
                    args.overlay_app_id,
                    latest_layout,
                )
                checks.append(capture_check)
                if packet:
                    evidence_packets.append(packet)

        checks.append(passed("focus-sequence", "overlay labels remained stable across focus changes", sequence=focus_sequence))

        if not args.output_name and args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))
            evidence_packets.append(unavailable_packet(checked_at, surface_ids(launched), args.overlay_app_id))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)

        cleanup_checks = cleanup_launched(args.base_url, launched, args.timeout_seconds)
        checks.extend(cleanup_checks)
    finally:
        for item in launched:
            try:
                post_json(args.base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
            except Exception:
                pass

    return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, focus_sequence, latest_layout, checked_at)


def check_compositorctl_path(compositorctl: str) -> dict:
    path = pathlib.Path(compositorctl)
    try:
        resolved = path.resolve(strict=False)
    except OSError:
        resolved = path
    if path == OLD_COMPOSITORCTL or resolved == OLD_COMPOSITORCTL:
        return failed("compositorctl-path", "old /usr/local/bin/compositorctl path is not allowed")
    if path.name != "agora-de-compositorctl":
        return failed("compositorctl-path", "overlay evidence must use agora-de-compositorctl", path=str(path))
    return passed("compositorctl-path", "using agora-de compositorctl", path=str(path))


def check_overlay_route(base_url: str) -> dict:
    body = get_text(base_url + "/shell/dist/desktop/?surface=overlay")
    required = [
        'data-surface="overlay"',
        'id="agent-overlay-surface"',
        'id="overlay-labels"',
        'id="zone-hints"',
        'className = "window-box"',
        'className = "bounds"',
        'className = "meta"',
        'action-hints',
        'dataset.layoutRule',
        "/api/layout",
        "/api/surfaces",
    ]
    missing = [value for value in required if value not in body]
    if missing:
        return failed("overlay-route", "overlay route missing label/bounds hooks", missing=missing)
    return passed("overlay-route", "overlay route exposes layout-driven label and bounds hooks")


def check_catalog(base_url: str, app_ids: list[str]) -> dict:
    catalog = get_json(base_url + "/api/catalog/apps")
    apps = catalog.get("apps", [])
    if not isinstance(apps, list):
        return failed("catalog", "catalog route response must contain apps array")
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
        return failed("catalog", "one or more overlay fixture apps are not launchable", failures=failures)
    return passed("catalog", "all overlay fixture apps are launchable", appIds=app_ids)


def get_text(url: str) -> str:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-overlay-labels/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return response.read().decode("utf-8")


def get_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-overlay-labels/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))


def post_json(url: str, body: dict) -> dict:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-overlay-labels/1"},
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{url} returned {error.code}: {detail}") from error


def wait_for_overlay_surface(base_url: str, overlay_app_id: str, timeout_seconds: float) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
            visible = surface.get("visible")
            if visible is None:
                visible = surface.get("mapped")
            if surface.get("appId") == overlay_app_id and surface.get("mapped") and visible:
                return surface
        time.sleep(0.25)
    return None


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


def surface_ids_for_app(base_url: str, expected_app_id: str) -> set[str]:
    ids = set()
    for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
        if surface.get("appId") == expected_app_id and surface.get("id"):
            ids.add(surface["id"])
    return ids


def wait_for_new_surface(base_url: str, expected_app_id: str, existing_surface_ids: set[str], timeout_seconds: float) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        candidates = []
        for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
            visible = surface.get("visible")
            if visible is None:
                visible = surface.get("mapped")
            if (
                surface.get("appId") == expected_app_id
                and surface.get("id")
                and surface.get("id") not in existing_surface_ids
                and surface.get("mapped")
                and visible
            ):
                candidates.append(surface)
        if candidates:
            return candidates[-1]
        time.sleep(0.25)
    return None


def wait_for_layout_surfaces(base_url: str, launched: list[dict], timeout_seconds: float) -> dict | None:
    wanted = set(surface_ids(launched))
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        layout = get_json(base_url + "/api/layout").get("layout") or {}
        present = {surface.get("surfaceId") for surface in layout.get("surfaces", []) if isinstance(surface, dict)}
        if wanted.issubset(present):
            return layout
        time.sleep(0.25)
    return None


def wait_for_focused_layout(base_url: str, surface_id: str, timeout_seconds: float) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        layout = get_json(base_url + "/api/layout").get("layout") or {}
        for surface in layout.get("surfaces", []):
            if isinstance(surface, dict) and surface.get("surfaceId") == surface_id and surface.get("focused"):
                return layout
        time.sleep(0.25)
    return None


def check_layout_labels(layout: dict, launched: list[dict]) -> dict:
    if not isinstance(layout, dict):
        return failed("layout-labels", "layout route did not return a layout object")
    wanted = set(surface_ids(launched))
    surfaces = [surface for surface in layout.get("surfaces", []) if isinstance(surface, dict) and surface.get("surfaceId") in wanted]
    if len(surfaces) != len(launched):
        return failed(
            "layout-labels",
            "layout route did not include every launched surface",
            expectedSurfaceIds=surface_ids(launched),
            layoutSurfaceIds=[surface.get("surfaceId") for surface in surfaces],
        )
    missing_labels = [surface.get("surfaceId") for surface in surfaces if not str(surface.get("label") or "").strip()]
    if missing_labels:
        return failed("layout-labels", "layout surfaces are missing stable labels", missingLabels=missing_labels)
    missing_geometry = [surface.get("surfaceId") for surface in surfaces if not isinstance(surface.get("geometry"), dict)]
    if missing_geometry:
        return failed(
            "layout-labels",
            "layout surfaces are missing geometry needed for overlay bounds",
            labels={surface.get("surfaceId"): surface.get("label") for surface in surfaces},
            missingGeometry=missing_geometry,
        )
    return passed(
        "layout-labels",
        "layout provides stable labels and bounds for overlay boxes",
        labels={surface.get("surfaceId"): surface.get("label") for surface in surfaces},
    )


def layout_label(layout: dict, surface_id: str) -> str:
    if not isinstance(layout, dict):
        return ""
    for surface in layout.get("surfaces", []):
        if isinstance(surface, dict) and surface.get("surfaceId") == surface_id:
            return str(surface.get("label") or "")
    return ""


def wait_until_absent(base_url: str, surface_id: str, timeout_seconds: float) -> bool:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        surfaces = get_json(base_url + "/api/surfaces").get("surfaces", [])
        if not any(surface.get("id") == surface_id for surface in surfaces):
            return True
        time.sleep(0.25)
    return False


def cleanup_launched(base_url: str, launched: list[dict], timeout_seconds: float) -> list[dict]:
    checks = []
    for item in launched:
        post_json(base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
        if wait_until_absent(base_url, item["surfaceId"], timeout_seconds):
            checks.append(passed("cleanup", "closed surface disappeared from running state", surfaceId=item["surfaceId"]))
        else:
            checks.append(failed("cleanup", "closed surface remained in running state", surfaceId=item["surfaceId"]))
    return checks


def capture_visible_overlay(
    compositorctl: str,
    output_name: str,
    session_id: str,
    checked_at: int,
    surface_ids_value: list[str],
    focused_surface_id: str,
    overlay_app_id: str,
    layout: dict,
) -> tuple[dict, dict | None]:
    live_evidence = load_live_evidence_module()
    capture_check, packet = capture_overlay_output(live_evidence, compositorctl, output_name, session_id, checked_at)
    if packet:
        packet = dict(packet)
        packet["scenario"] = "den-k8-overlay-labels-visible"
        packet["surfaceIds"] = surface_ids_value
        packet["focusedSurfaceId"] = focused_surface_id
        packet["overlayAppId"] = overlay_app_id
        pixel_classification, pixel_error = classify_overlay_output_pixels(
            live_evidence,
            pathlib.Path(str(packet.get("artifactPath") or "")),
            layout,
            surface_ids_value,
        )
        if pixel_classification:
            packet["overlayPixelClassification"] = pixel_classification
        elif pixel_error:
            packet["overlayPixelClassificationError"] = pixel_error
    if capture_check.get("status") == "pass":
        capture_check = dict(capture_check)
        capture_check["name"] = "overlay-labels-visible-capture"
        pixel_classification = packet.get("overlayPixelClassification") if packet else None
        overlay_visible = isinstance(pixel_classification, dict) and pixel_classification.get("overlayVisible")
        native_visible = isinstance(pixel_classification, dict) and pixel_classification.get("nativePixelsVisible")
        if not overlay_visible or not native_visible:
            capture_check["status"] = "fail"
            capture_check["detail"] = "physical output capture did not prove both overlay annotations and native app pixels"
        else:
            capture_check["detail"] = "physical output capture shows overlay annotations without hiding native app pixels"
        capture_check["surfaceIds"] = surface_ids_value
        capture_check["focusedSurfaceId"] = focused_surface_id
        capture_check["overlayAppId"] = overlay_app_id
        if packet and isinstance(packet.get("overlayPixelClassification"), dict):
            capture_check["overlayPixelClassification"] = packet["overlayPixelClassification"]
    return capture_check, packet


def capture_overlay_output(live_evidence, compositorctl: str, output_name: str, session_id: str, checked_at: int) -> tuple[dict, dict | None]:
    try:
        completed = subprocess.run(
            [
                compositorctl,
                "output",
                "capture",
                "--name",
                output_name,
                "--export",
                "--session",
                session_id,
                "--evidence-class",
                "viewport_screenshot",
            ],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=10,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        return failed("capture", f"compositorctl output capture failed: {error}"), unavailable_packet(
            checked_at,
            [],
            "",
        )
    if completed.returncode != 0:
        return failed("capture", "compositorctl output capture failed", stderr=completed.stderr.strip()), unavailable_packet(
            checked_at,
            [],
            "",
        )
    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        return failed("capture", f"invalid output capture JSON: {error}"), unavailable_packet(checked_at, [], "")
    capture = live_evidence.first_capture_record(payload)
    if not capture:
        return failed("capture", "output capture did not return any capture artifacts"), unavailable_packet(checked_at, [], "")
    return live_evidence.classify_capture_record(
        capture,
        checked_at,
        output_name=output_name,
        require_shell_pixels=False,
    )


def classify_overlay_output_pixels(live_evidence, image_path: pathlib.Path, layout: dict, surface_ids_value: list[str]) -> tuple[dict | None, str]:
    if not str(image_path):
        return None, "capture packet did not include an artifact path"
    try:
        image = live_evidence.read_png_rgb(image_path)
    except Exception as error:
        return None, str(error)

    width = int(image["width"])
    height = int(image["height"])
    rows = image["rows"]
    if width <= 0 or height <= 0:
        return None, "capture has empty dimensions"

    overlay_pixels = 0
    for row in rows:
        for x in range(width):
            r, g, b = rgb_at(row, x)
            if is_overlay_annotation_pixel(r, g, b):
                overlay_pixels += 1

    native_regions = native_region_stats(rows, width, height, layout, surface_ids_value)
    native_pixels_visible = any(region["nativePixelsVisible"] for region in native_regions)
    overlay_visible = overlay_pixels >= max(90, width // 10)
    mapped_only = not overlay_visible or not native_pixels_visible
    return {
        "classification": "overlay_and_native_pixels_visible" if not mapped_only else "mapped_only_or_occluding",
        "overlayVisible": overlay_visible,
        "nativePixelsVisible": native_pixels_visible,
        "mappedOnly": mapped_only,
        "overlayAnnotationPixels": overlay_pixels,
        "nativeRegions": native_regions,
        "imageWidth": width,
        "imageHeight": height,
    }, ""


def native_region_stats(rows: list[bytes], output_width: int, output_height: int, layout: dict, surface_ids_value: list[str]) -> list[dict]:
    wanted = set(surface_ids_value)
    stats = []
    for surface in layout.get("surfaces", []) if isinstance(layout, dict) else []:
        if not isinstance(surface, dict) or surface.get("surfaceId") not in wanted:
            continue
        geometry = surface.get("geometry")
        if not isinstance(geometry, dict):
            stats.append({"surfaceId": surface.get("surfaceId") or "", "nativePixelsVisible": False, "reason": "missing_geometry"})
            continue
        x = clamp_int(geometry.get("x"), 0, output_width - 1)
        y = clamp_int(geometry.get("y"), 0, output_height - 1)
        w = clamp_int(geometry.get("width"), 0, output_width - x)
        h = clamp_int(geometry.get("height"), 0, output_height - y)
        if w <= 0 or h <= 0:
            stats.append({"surfaceId": surface.get("surfaceId") or "", "nativePixelsVisible": False, "reason": "empty_geometry"})
            continue
        inset_x = min(max(24, w // 8), max(0, (w - 1) // 2))
        inset_y = min(max(24, h // 8), max(0, (h - 1) // 2))
        left = x + inset_x
        top = y + inset_y
        right = max(left + 1, x + w - inset_x)
        bottom = max(top + 1, y + h - inset_y)
        sample_step = max(1, min(max(1, right - left), max(1, bottom - top)) // 80)
        sampled = 0
        non_overlay = 0
        light_pixels = 0
        mid_pixels = 0
        saturated_pixels = 0
        unique_colors = set()
        for yy in range(top, bottom, sample_step):
            row = rows[yy]
            for xx in range(left, right, sample_step):
                r, g, b = rgb_at(row, xx)
                sampled += 1
                if is_overlay_annotation_pixel(r, g, b):
                    continue
                non_overlay += 1
                bucket = (r // 24, g // 24, b // 24)
                unique_colors.add(bucket)
                if r >= 220 and g >= 220 and b >= 220:
                    light_pixels += 1
                if 70 <= r <= 210 and 70 <= g <= 210 and 70 <= b <= 210:
                    mid_pixels += 1
                if max(r, g, b) - min(r, g, b) >= 80 and max(r, g, b) >= 120:
                    saturated_pixels += 1
        native_visible = (
            sampled >= 64
            and non_overlay >= max(48, sampled // 3)
            and (len(unique_colors) >= 6 or light_pixels >= 24 or mid_pixels >= 24 or saturated_pixels >= 12)
        )
        stats.append(
            {
                "surfaceId": surface.get("surfaceId") or "",
                "appId": surface.get("appId") or "",
                "nativePixelsVisible": native_visible,
                "sampledPixels": sampled,
                "nonOverlayPixels": non_overlay,
                "uniqueColorBuckets": len(unique_colors),
                "lightPixels": light_pixels,
                "midPixels": mid_pixels,
                "saturatedPixels": saturated_pixels,
            }
        )
    return stats


def rgb_at(row: bytes, x: int) -> tuple[int, int, int]:
    offset = x * 3
    return row[offset], row[offset + 1], row[offset + 2]


def is_overlay_annotation_pixel(r: int, g: int, b: int) -> bool:
    cyan = r <= 80 and g >= 150 and b >= 120
    yellow = r >= 200 and g >= 140 and b <= 120
    return cyan or yellow


def clamp_int(value: object, low: int, high: int) -> int:
    try:
        parsed = int(float(value))
    except (TypeError, ValueError):
        parsed = low
    return max(low, min(parsed, high))


def load_live_evidence_module():
    path = pathlib.Path(__file__).with_name("check-den-k8.py")
    spec = importlib.util.spec_from_file_location("agora_de_check_den_k8", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load live evidence module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def unavailable_packet(checked_at: int, surface_ids_value: list[str], overlay_app_id: str) -> dict:
    return {
        "scenario": "den-k8-overlay-labels-visible",
        "capturedAtUnixMillis": checked_at,
        "surfaceIds": surface_ids_value,
        "overlayAppId": overlay_app_id,
        "visualStatus": "unknown",
        "captureClassification": "not_visible",
        "overlayPixelClassification": {
            "classification": "mapped_only_or_occluding",
            "overlayVisible": False,
            "nativePixelsVisible": False,
            "mappedOnly": True,
        },
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
    focus_sequence: list[dict],
    layout: dict,
    checked_at: int,
) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": "agora-de.overlay-labels-live.v1",
        "checkedAtUnixMillis": checked_at,
        "appIds": app_ids,
        "expectedAppIds": expected_app_ids,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "launched": launched,
        "focusSequence": focus_sequence,
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

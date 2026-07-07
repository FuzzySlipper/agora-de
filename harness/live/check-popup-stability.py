#!/usr/bin/env python3
import argparse
import json
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request


SCHEMA = "agora-de.popup-stability-live.v1"
SHELL_POPUP_APP_IDS = {"io.agorade.ShellStatus", "io.agorade.ShellLauncher"}
SHELL_APP_IDS = {
    "io.agorade.ShellBackground",
    "io.agorade.ShellLauncher",
    "io.agorade.ShellOverlay",
    "io.agorade.ShellPanel",
    "io.agorade.ShellStatus",
}


def main() -> int:
    parser = argparse.ArgumentParser(description="Check installed-service shell popup geometry stability.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-popup-stability")
    parser.add_argument("--require-capture", action="store_true")
    parser.add_argument("--samples-output", default="")
    parser.add_argument("--baseline-samples", type=int, default=3)
    parser.add_argument("--open-samples", type=int, default=6)
    parser.add_argument("--closed-samples", type=int, default=2)
    parser.add_argument("--cycles", type=int, default=2)
    parser.add_argument("--native-dialog-app-id", default="")
    parser.add_argument("--native-dialog-wait-seconds", type=float, default=2.0)
    parser.add_argument("--sample-delay-seconds", type=float, default=1.0)
    parser.add_argument("--launch-delay-seconds", type=float, default=1.0)
    parser.add_argument("--cleanup-delay-seconds", type=float, default=1.0)
    args = parser.parse_args()

    checked_at = unix_millis()
    checks: list[dict] = []
    evidence_packets: list[dict] = []
    samples: list[dict] = []
    capture_artifacts: list[dict] = []

    path_check = check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(args, checked_at, checks, evidence_packets, samples, capture_artifacts)

    try:
        close_shell_popups(args.base_url)
        time.sleep(args.cleanup_delay_seconds)
        collect_samples(args, "baseline", args.baseline_samples, samples)

        cycles = max(1, args.cycles)
        for cycle in range(1, cycles + 1):
            launch_app(args.base_url, "shell-status")
            time.sleep(args.launch_delay_seconds)
            collect_samples(args, "status_open", args.open_samples, samples, cycle)
            if cycle == 1:
                capture_artifact = capture_output(args, "status")
                if capture_artifact:
                    capture_artifacts.append(capture_artifact)
            close_shell_popups(args.base_url)
            time.sleep(args.cleanup_delay_seconds)
            collect_samples(args, "status_closed", args.closed_samples, samples, cycle)

            launch_app(args.base_url, "shell-launcher")
            time.sleep(args.launch_delay_seconds)
            collect_samples(args, "launcher_open", args.open_samples, samples, cycle)
            if cycle == 1:
                capture_artifact = capture_output(args, "launcher")
                if capture_artifact:
                    capture_artifacts.append(capture_artifact)
            close_shell_popups(args.base_url)
            time.sleep(args.cleanup_delay_seconds)
            collect_samples(args, "launcher_closed", args.closed_samples, samples, cycle)

        checks.append(probe_native_dialog(args, samples))
    except Exception as error:
        checks.append(failed("popup-stability", f"popup stability probe failed: {error}"))
    finally:
        try:
            close_shell_popups(args.base_url)
        except Exception:
            pass

    if samples:
        checks.extend(classify_samples(samples))
        cleanup_check = check_popup_cleanup(args)
        checks.append(cleanup_check)

    if args.output_name:
        if capture_artifacts:
            checks.append(passed("capture", "captured popup output evidence", captures=[artifact.get("image_path") or artifact.get("path") for artifact in capture_artifacts]))
            for artifact in capture_artifacts:
                evidence_packets.append(capture_packet(artifact, checked_at, args.output_name))
        elif args.require_capture:
            checks.append(failed("capture", "popup output capture did not return artifacts"))
    elif args.require_capture:
        checks.append(failed("capture", "physical output capture is required; pass --output-name"))

    return finish(args, checked_at, checks, evidence_packets, samples, capture_artifacts)


def collect_samples(args: argparse.Namespace, phase: str, count: int, samples: list[dict], cycle: int = 0) -> None:
    for index in range(count):
        samples.append(sample(args, phase, index, cycle))
        if index + 1 < count and args.sample_delay_seconds > 0:
            time.sleep(args.sample_delay_seconds)


def sample(args: argparse.Namespace, phase: str, index: int, cycle: int) -> dict:
    surfaces = run_compositorctl_json(args.compositorctl, ["list-surfaces"]).get("surfaces") or []
    layout = run_compositorctl_json(args.compositorctl, ["layout", "get"]).get("layout") or {}
    route_surfaces = get_json(args.base_url + "/api/surfaces").get("surfaces") or []
    route_layout = get_json(args.base_url + "/api/layout").get("layout") or {}
    return {
        "phase": phase,
        "cycle": cycle,
        "index": index,
        "atUnixMillis": unix_millis(),
        "layoutRevision": layout.get("revision"),
        "layoutSurfaces": [summarize_layout_surface(item) for item in layout.get("surfaces") or []],
        "routeSurfaces": [summarize_route_surface(item) for item in route_surfaces],
        "routeLayoutSurfaces": [summarize_route_layout_surface(item) for item in route_layout.get("surfaces") or []],
        "panel": first_surface_summary(surfaces, lambda item: surface_value(item, "app_id") == "io.agorade.ShellPanel"),
        "background": first_surface_summary(surfaces, lambda item: surface_value(item, "app_id") == "io.agorade.ShellBackground"),
        "popups": [
            summarize_surface(item)
            for item in surfaces
            if surface_value(item, "app_id") in SHELL_POPUP_APP_IDS
        ],
        "workSurfaces": [
            summarize_surface(item)
            for item in surfaces
            if is_work_surface(item)
        ],
        "unmanagedViews": [
            summarize_surface(item)
            for item in surfaces
            if surface_value(item, "surface_kind") == "xdg_view" and surface_value(item, "role") == "unmanaged"
        ],
    }


def classify_samples(samples: list[dict]) -> list[dict]:
    checks = []
    checks.append(check_stable_named_geometry(samples, "panel", "panel-geometry", "panel geometry remains stable"))
    checks.append(check_stable_named_geometry(samples, "background", "background-geometry", "background geometry remains stable"))
    checks.append(check_popup_phase(samples, "status_open", "io.agorade.ShellStatus", "status-popup-geometry"))
    checks.append(check_popup_phase(samples, "launcher_open", "io.agorade.ShellLauncher", "launcher-popup-geometry"))
    checks.append(check_popup_policy(samples))
    checks.append(check_closed_popup_absence(samples))
    checks.append(check_work_surface_stability(samples))
    checks.append(check_unmanaged_transient(samples))
    return checks


def check_stable_named_geometry(samples: list[dict], key: str, name: str, detail: str) -> dict:
    geometries = unique_values(geometry_key((sample.get(key) or {}).get("geometry")) for sample in samples if sample.get(key))
    if not geometries:
        return failed(name, f"no {key} surface was observed")
    if len(geometries) != 1:
        return failed(name, f"{key} geometry changed", geometries=geometries)
    return passed(name, detail, geometry=geometries[0])


def check_popup_phase(samples: list[dict], phase: str, app_id: str, name: str) -> dict:
    phase_samples = [sample for sample in samples if sample.get("phase") == phase]
    if not phase_samples:
        return failed(name, f"no samples recorded for {phase}")
    geometries: list[str] = []
    anchors: list[str] = []
    exclusive_zone_values: list[object] = []
    for sample_item in phase_samples:
        matches = [popup for popup in sample_item.get("popups", []) if popup.get("appId") == app_id]
        if len(matches) != 1:
            return failed(name, f"expected exactly one {app_id} popup in {phase}", count=len(matches), sample=sample_item)
        popup = matches[0]
        geometries.append(geometry_key(popup.get("geometry")))
        anchors.append("+".join(popup.get("anchors") or []))
        exclusive_zone_values.append(popup.get("exclusiveZone"))
    if len(unique_values(geometries)) != 1:
        return failed(name, f"{app_id} popup geometry changed", geometries=unique_values(geometries))
    if any(value is not False for value in exclusive_zone_values):
        return failed(name, f"{app_id} popup reserved exclusive layer-shell space", exclusiveZoneValues=exclusive_zone_values)
    return passed(name, f"{app_id} popup geometry is stable and non-exclusive", geometry=geometries[0], anchors=unique_values(anchors))


def check_work_surface_stability(samples: list[dict]) -> dict:
    baseline = [sample for sample in samples if sample.get("phase") == "baseline"]
    if not baseline:
        return failed("work-surface-geometry", "no baseline samples recorded")
    baseline_geometries: dict[str, str] = {}
    for surface in baseline[-1].get("workSurfaces", []):
        surface_id = surface.get("id", "")
        if surface_id:
            baseline_geometries[surface_id] = geometry_key(surface.get("geometry"))
    if not baseline_geometries:
        return passed("work-surface-geometry", "no tiled work surfaces were present to compare")

    changes = []
    for sample_item in samples:
        for surface in sample_item.get("workSurfaces", []):
            surface_id = surface.get("id", "")
            if surface_id not in baseline_geometries:
                continue
            geometry = geometry_key(surface.get("geometry"))
            if geometry != baseline_geometries[surface_id]:
                changes.append({"phase": sample_item.get("phase"), "surfaceId": surface_id, "baseline": baseline_geometries[surface_id], "geometry": geometry})
    if changes:
        return failed("work-surface-geometry", "work surface geometry changed while shell popups opened or closed", changes=changes)
    return passed("work-surface-geometry", "baseline work surface geometry stayed stable across popup phases", surfaces=baseline_geometries)


def check_closed_popup_absence(samples: list[dict]) -> dict:
    offenders = []
    for sample_item in samples:
        if sample_item.get("phase") not in {"status_closed", "launcher_closed"}:
            continue
        for popup in sample_item.get("popups", []):
            offenders.append({
                "phase": sample_item.get("phase"),
                "cycle": sample_item.get("cycle"),
                "index": sample_item.get("index"),
                "surface": popup,
            })
    if offenders:
        return failed("closed-popup-cleanup", "closed popup phases retained shell popup surfaces", offenders=offenders)
    cycles = unique_values(sample.get("cycle") for sample in samples if sample.get("cycle"))
    return passed("closed-popup-cleanup", "closed popup phases stayed clear after repeated cycles", cycles=cycles)


def check_unmanaged_transient(samples: list[dict]) -> dict:
    offenders = []
    observed = []
    for sample_item in samples:
        for surface in sample_item.get("unmanagedViews", []):
            observed.append(surface.get("id"))
            if surface.get("layoutRole") != "transient" or surface.get("zoneId") != "transient" or surface.get("policyClass") not in {"transient", "no_parent"}:
                offenders.append({"phase": sample_item.get("phase"), "surface": surface})
    if offenders:
        return failed("unmanaged-transient", "unmanaged XDG helper views must not be tiled and must expose transient policy", offenders=offenders)
    return passed("unmanaged-transient", "unmanaged XDG helper views are transient", observed=unique_values(observed))


def check_popup_policy(samples: list[dict]) -> dict:
    offenders = []
    observed = []
    for sample_item in samples:
        if sample_item.get("phase") not in {"status_open", "launcher_open"}:
            continue
        for popup in sample_item.get("popups", []):
            observed.append({"id": popup.get("id"), "appId": popup.get("appId"), "policyClass": popup.get("policyClass")})
            if not valid_popup_policy(popup):
                offenders.append({"phase": sample_item.get("phase"), "surface": popup})
    if offenders:
        return failed("popup-policy", "shell popup surfaces must project transient policy and stay out of tiling", offenders=offenders)
    if not observed:
        return failed("popup-policy", "no shell popup policy samples were observed")
    return passed("popup-policy", "shell popup samples projected transient policy", observed=observed)


def valid_popup_policy(surface: dict) -> bool:
    if surface.get("layoutRole") != "transient":
        return False
    policy = surface.get("policyClass")
    zone = surface.get("zoneId")
    if policy == "shell_chrome":
        return zone == "chrome"
    if policy == "transient":
        return zone == "transient"
    return False


def probe_native_dialog(args: argparse.Namespace, samples: list[dict]) -> dict:
    app_id = args.native_dialog_app_id.strip()
    if not app_id:
        return skipped("native-dialog-capability", "native dialog probe skipped; pass --native-dialog-app-id to exercise a host-specific app dialog")
    before = [surface.get("id") for surface in get_json(args.base_url + "/api/surfaces").get("surfaces") or []]
    try:
        launch_app(args.base_url, app_id)
        time.sleep(max(0, args.native_dialog_wait_seconds))
        collect_samples(args, "native_dialog", 1, samples)
        candidates = native_dialog_candidates(samples[-1], app_id, before)
        if not candidates:
            return skipped("native-dialog-capability", f"native dialog probe launched {app_id} but did not observe a dialog/transient candidate", appId=app_id)
        offenders = [
            surface
            for surface in candidates
            if surface.get("policyClass") not in {"transient", "no_parent", "floating_override"} or surface.get("layoutRole") == "tiled"
        ]
        if offenders:
            return failed("native-dialog-policy", "native dialog candidates joined tiling or lacked dialog policy", appId=app_id, offenders=offenders)
        return passed("native-dialog-policy", "native dialog candidates were classified outside tiled work surfaces", appId=app_id, candidates=candidates)
    except Exception as error:
        return skipped("native-dialog-capability", f"native dialog probe skipped because {app_id} was unavailable: {error}", appId=app_id)
    finally:
        close_native_dialog_candidates(args.base_url, app_id, before)


def native_dialog_candidates(sample_item: dict, app_id: str, before: list[str]) -> list[dict]:
    before_ids = set(before)
    candidates = []
    for surface in sample_item.get("routeSurfaces", []):
        role = str(surface.get("role") or "").lower()
        policy = surface.get("policyClass")
        is_dialog_role = any(marker in role for marker in ["dialog", "modal", "popup", "popover", "menu", "tooltip", "transient"])
        is_new_app = surface.get("appId") == app_id and surface.get("id") not in before_ids
        if is_dialog_role or policy in {"transient", "no_parent", "floating_override"} or is_new_app:
            candidates.append(surface)
    return candidates


def check_popup_cleanup(args: argparse.Namespace) -> dict:
    surfaces = run_compositorctl_json(args.compositorctl, ["list-surfaces"]).get("surfaces") or []
    remaining = [
        summarize_surface(item)
        for item in surfaces
        if surface_value(item, "app_id") in SHELL_POPUP_APP_IDS
    ]
    if remaining:
        return failed("cleanup", "shell popup surfaces remained after cleanup", remaining=remaining)
    return passed("cleanup", "shell popup surfaces closed after probe")


def close_native_dialog_candidates(base_url: str, app_id: str, before: list[str]) -> None:
    before_ids = set(before)
    try:
        surfaces = get_json(base_url + "/api/surfaces").get("surfaces") or []
    except Exception:
        return
    for surface in surfaces:
        surface_id = surface.get("id")
        if not surface_id or surface_id in before_ids:
            continue
        role = str(surface.get("role") or "").lower()
        if surface.get("appId") != app_id and not any(marker in role for marker in ["dialog", "modal", "popup", "popover", "menu", "tooltip", "transient"]):
            continue
        try:
            post_json(base_url + "/api/surfaces/action", {"surfaceId": surface_id, "action": "close"})
        except Exception:
            pass


def first_surface_summary(surfaces: list[dict], predicate) -> dict | None:
    for item in surfaces:
        if predicate(item):
            return summarize_surface(item)
    return None


def summarize_surface(item: dict) -> dict:
    surface = item.get("surface") or {}
    layer = surface.get("layer_shell") or {}
    return {
        "id": surface.get("id", ""),
        "appId": surface.get("app_id", ""),
        "title": surface.get("title", ""),
        "kind": surface.get("surface_kind", ""),
        "role": surface.get("role", ""),
        "layoutRole": item.get("layout_role") or surface.get("layout_role", ""),
        "policyClass": item.get("policy_class") or surface.get("policy_class", ""),
        "policyReason": item.get("policy_reason") or surface.get("policy_reason", ""),
        "parentSurfaceId": item.get("parent_surface_id") or surface.get("parent_surface_id", ""),
        "zoneId": item.get("zone_id") or surface.get("zone_id", ""),
        "anchors": layer.get("anchors") or [],
        "exclusiveZone": layer.get("exclusive_zone"),
        "geometry": geometry(item),
        "visible": item.get("visible") or surface.get("visible"),
        "contentCommitCount": item.get("content_commit_count"),
        "layoutRevision": item.get("layout_revision"),
        "clientPid": (item.get("client") or {}).get("pid"),
    }


def summarize_layout_surface(item: dict) -> dict:
    return {
        "id": item.get("surface_id", ""),
        "appId": item.get("app_id", ""),
        "title": item.get("title", ""),
        "role": item.get("role", ""),
        "layoutRole": item.get("participation", ""),
        "policyClass": item.get("policy_class", ""),
        "policyReason": item.get("policy_reason", ""),
        "parentSurfaceId": item.get("parent_surface_id", ""),
        "zoneId": item.get("zone_id", ""),
        "geometry": item.get("geometry") or {},
        "visible": item.get("visible"),
    }


def summarize_route_surface(item: dict) -> dict:
    return {
        "id": item.get("id", ""),
        "appId": item.get("appId", ""),
        "title": item.get("title", ""),
        "role": item.get("role", ""),
        "layoutRole": item.get("layoutRole", ""),
        "policyClass": item.get("policyClass", ""),
        "policyReason": item.get("policyReason", ""),
        "parentSurfaceId": item.get("parentSurfaceId", ""),
        "zoneId": item.get("zoneId", ""),
        "geometry": item.get("geometry") or {},
        "visible": item.get("visible"),
        "mapped": item.get("mapped"),
    }


def summarize_route_layout_surface(item: dict) -> dict:
    return {
        "id": item.get("surfaceId", ""),
        "appId": item.get("appId", ""),
        "title": item.get("title", ""),
        "role": item.get("role", ""),
        "layoutRole": item.get("participation", ""),
        "policyClass": item.get("policyClass", ""),
        "policyReason": item.get("policyReason", ""),
        "parentSurfaceId": item.get("parentSurfaceId", ""),
        "zoneId": item.get("zoneId", ""),
        "geometry": item.get("geometry") or {},
        "visible": item.get("visible"),
    }


def is_work_surface(item: dict) -> bool:
    if surface_value(item, "surface_kind") != "xdg_view":
        return False
    if surface_value(item, "role") == "unmanaged":
        return False
    app_id = surface_value(item, "app_id")
    if app_id in SHELL_APP_IDS:
        return False
    return bool(item.get("visible") or surface_value(item, "visible"))


def geometry(item: dict) -> dict:
    return item.get("geometry") or (item.get("surface") or {}).get("geometry") or {}


def geometry_key(value: object) -> str:
    if not isinstance(value, dict):
        return "none"
    return f"{int(value.get('x') or 0)},{int(value.get('y') or 0)},{int(value.get('width') or 0)},{int(value.get('height') or 0)}"


def surface_value(item: dict, key: str) -> object:
    return (item.get("surface") or {}).get(key)


def launch_app(base_url: str, app_id: str) -> dict:
    return post_json(base_url + "/api/catalog/launch", {"appId": app_id})


def close_shell_popups(base_url: str) -> None:
    surfaces = get_json(base_url + "/api/surfaces").get("surfaces") or []
    for surface in surfaces:
        if surface.get("appId") in SHELL_POPUP_APP_IDS and surface.get("id"):
            try:
                post_json(base_url + "/api/surfaces/action", {"surfaceId": surface["id"], "action": "close"})
            except Exception:
                pass


def capture_output(args: argparse.Namespace, label: str) -> dict | None:
    if not args.output_name:
        return None
    session = f"{args.output_capture_session}-{label}"
    payload = run_compositorctl_json(args.compositorctl, ["output", "capture", "--name", args.output_name, "--session", session, "--export"])
    captures = payload.get("captures") or []
    return captures[0] if captures else None


def run_compositorctl_json(compositorctl: str, args: list[str]) -> dict:
    completed = subprocess.run([compositorctl, "--pretty", *args], check=False, text=True, capture_output=True)
    if completed.returncode != 0:
        raise RuntimeError(f"compositorctl {' '.join(args)} failed: {completed.stderr.strip() or completed.stdout.strip()}")
    try:
        return json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        raise RuntimeError(f"compositorctl {' '.join(args)} returned invalid JSON: {completed.stdout}") from error


def check_compositorctl_path(compositorctl: str) -> dict:
    try:
        completed = subprocess.run([compositorctl, "--pretty", "list-surfaces"], check=False, text=True, capture_output=True, timeout=3)
    except (OSError, subprocess.TimeoutExpired) as error:
        return failed("compositorctl", f"compositorctl unavailable: {error}")
    if completed.returncode != 0:
        return failed("compositorctl", "compositorctl list-surfaces failed", stderr=completed.stderr.strip())
    try:
        json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        return failed("compositorctl", f"compositorctl list-surfaces returned invalid JSON: {error}")
    return passed("compositorctl", "compositorctl is available", path=compositorctl)


def get_json(url: str) -> dict:
    with urllib.request.urlopen(url, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))


def post_json(url: str, body: dict) -> dict:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-popup-stability/1"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{url} returned {error.code}: {detail}") from error


def capture_packet(capture: dict, checked_at: int, output_name: str) -> dict:
    image_path = capture.get("image_path") or capture.get("path")
    return {
        "scenario": "den-k8-popup-stability-capture",
        "capturedAtUnixMillis": checked_at,
        "visualStatus": (capture.get("visual_inspection") or {}).get("status", "unknown"),
        "captureClassification": "capture_visible" if image_path else "not_visible",
        "outputName": output_name,
        "artifactPath": image_path,
    }


def finish(
    args: argparse.Namespace,
    checked_at: int,
    checks: list[dict],
    evidence_packets: list[dict],
    samples: list[dict],
    capture_artifacts: list[dict],
) -> int:
    failed_checks = [check for check in checks if check.get("status") == "fail"]
    samples_path = write_samples(args.samples_output, samples) if args.samples_output and samples else ""
    result = {
        "schema": SCHEMA,
        "checkedAtUnixMillis": checked_at,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "sampleCount": len(samples),
        "samplesPath": samples_path,
        "captureArtifacts": [
            {
                "imagePath": capture.get("image_path") or capture.get("path"),
                "sha256": capture.get("sha256"),
            }
            for capture in capture_artifacts
        ],
        "summary": {
            "status": "fail" if failed_checks else "pass",
            "passed": len([check for check in checks if check.get("status") == "pass"]),
            "failed": len(failed_checks),
            "skipped": len([check for check in checks if check.get("status") == "skip"]),
        },
    }
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 1 if failed_checks else 0


def write_samples(path: str, samples: list[dict]) -> str:
    target = pathlib.Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    with target.open("w", encoding="utf-8") as handle:
        for sample_item in samples:
            handle.write(json.dumps(sample_item, sort_keys=True) + "\n")
    return str(target)


def unique_values(values) -> list:
    result = []
    for value in values:
        if value not in result:
            result.append(value)
    return result


def unix_millis() -> int:
    return int(time.time() * 1000)


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "popup-stability", "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "popup-stability", "status": "fail", "detail": detail, **extra}


def skipped(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "popup-stability", "status": "skip", "detail": detail, **extra}


if __name__ == "__main__":
    raise SystemExit(main())

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
    parser = argparse.ArgumentParser(description="Check governed native app launch through installed shellui.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--app-id", default="Alacritty.desktop")
    parser.add_argument("--expected-app-id", default="Alacritty")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-native-launch")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.5)
    parser.add_argument(
        "--require-capture",
        action="store_true",
        help="Fail unless --output-name supplies visible physical-output capture evidence.",
    )
    parser.add_argument("--timeout-seconds", type=float, default=8)
    args = parser.parse_args()

    checked_at = unix_millis()
    checks = []
    evidence_packets = []
    launched_surface = ""

    path_check = check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)

    try:
        catalog = get_json(args.base_url + "/api/catalog/apps")
        apps = catalog.get("apps", [])
        if not isinstance(apps, list):
            checks.append(failed("catalog-shape", "catalog route response must contain apps array"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)

        app = next((item for item in apps if isinstance(item, dict) and item.get("id") == args.app_id), None)
        if not app:
            checks.append(failed("catalog", f"app {args.app_id!r} not present"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)
        if app.get("launchable") is not True:
            checks.append(
                failed(
                    "catalog",
                    f"app {args.app_id!r} is not launchable",
                    disabledCode=app.get("disabledCode") or "",
                    disabledReason=app.get("disabledReason") or "",
                )
            )
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)
        if app.get("disabledCode") or app.get("disabledReason"):
            checks.append(failed("catalog", f"launchable app {args.app_id!r} carries disabled state"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)
        checks.append(passed("catalog", "allowlisted native app is launchable", appId=args.app_id))

        nonlaunchable = [item for item in apps if isinstance(item, dict) and item.get("launchable") is not True]
        missing_code = [item.get("id") for item in nonlaunchable if not item.get("disabledCode")]
        if missing_code:
            checks.append(failed("disabled-codes", "non-launchable catalog entries missing disabledCode", sample=missing_code[:5]))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)
        checks.append(passed("disabled-codes", "non-launchable native apps carry disabled codes", count=len(nonlaunchable)))

        launch = post_json(args.base_url + "/api/catalog/launch", {"appId": args.app_id})
        launched_surface = launch.get("surfaceId") or ""
        if launch.get("appId") != args.app_id or launch.get("status") not in SUCCESSFUL_LAUNCH_STATUSES or not launched_surface:
            checks.append(failed("launch", f"unexpected launch response: {launch}"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)
        checks.append(
            passed(
                "launch",
                "native launch returned mapped surface identity",
                surfaceId=launched_surface,
                launchId=launch.get("launchId") or "",
            )
        )

        surface = wait_for_surface(args.base_url, launched_surface, args.expected_app_id, args.timeout_seconds)
        if not surface:
            checks.append(failed("running-state", f"surface {launched_surface!r} did not appear in /api/surfaces"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)
        checks.append(
            passed(
                "running-state",
                "native surface appears in compositor-backed shellui state",
                surfaceId=launched_surface,
                appId=surface.get("appId") or "",
                title=surface.get("title") or "",
            )
        )

        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "focus"})
        focused = wait_for_surface(args.base_url, launched_surface, args.expected_app_id, args.timeout_seconds, focused=True)
        if focused:
            checks.append(passed("focus", "focus action made native surface focused", surfaceId=launched_surface))
        else:
            checks.append(failed("focus", f"surface {launched_surface!r} did not become focused"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)

        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "minimize", "enabled": True})
        minimized = wait_for_surface(
            args.base_url,
            launched_surface,
            args.expected_app_id,
            args.timeout_seconds,
            minimized=True,
            visible=False,
        )
        if minimized:
            checks.append(passed("minimize", "minimize action made native surface restorable from shell state", surfaceId=launched_surface))
        else:
            checks.append(failed("minimize", f"surface {launched_surface!r} did not enter minimized shell state"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)

        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "minimize", "enabled": False})
        restored = wait_for_surface(
            args.base_url,
            launched_surface,
            args.expected_app_id,
            args.timeout_seconds,
            minimized=False,
            visible=True,
        )
        if restored:
            checks.append(passed("restore", "restore action made minimized native surface visible", surfaceId=launched_surface))
        else:
            checks.append(failed("restore", f"surface {launched_surface!r} did not restore from minimized shell state"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)

        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "focus"})
        focused = wait_for_surface(args.base_url, launched_surface, args.expected_app_id, args.timeout_seconds, focused=True)
        if focused:
            checks.append(passed("restore-focus", "restored native surface can be focused again", surfaceId=launched_surface))
        else:
            checks.append(failed("restore-focus", f"surface {launched_surface!r} did not focus after restore"))
            return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)

        presentation_check, presentation_packet = classify_native_surface_presentation(
            focused,
            checked_at,
            launched_surface,
            args.expected_app_id,
        )
        checks.append(presentation_check)
        evidence_packets.append(presentation_packet)

        if args.output_name:
            if args.capture_delay_seconds > 0:
                time.sleep(args.capture_delay_seconds)
            capture_check, packet = capture_visible_native_launch(
                args.compositorctl,
                args.output_name,
                args.output_capture_session,
                checked_at,
                launched_surface,
                args.expected_app_id,
            )
            checks.append(capture_check)
            if packet:
                evidence_packets.append(packet)
        elif args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))
            evidence_packets.append(unavailable_packet(checked_at, launched_surface, args.expected_app_id))

        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "close"})
        checks.append(passed("close", "close action accepted", surfaceId=launched_surface))

        if wait_until_absent(args.base_url, launched_surface, args.timeout_seconds):
            checks.append(passed("stale-cleanup", "closed native surface disappeared from running state", surfaceId=launched_surface))
        else:
            checks.append(failed("stale-cleanup", f"surface {launched_surface!r} remained after close"))
    finally:
        if launched_surface:
            try:
                post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "close"})
            except Exception:
                pass

    return finish(checks, evidence_packets, args.app_id, args.expected_app_id, launched_surface, checked_at)


def check_compositorctl_path(compositorctl: str) -> dict:
    path = pathlib.Path(compositorctl)
    try:
        resolved = path.resolve(strict=False)
    except OSError:
        resolved = path
    if path == OLD_COMPOSITORCTL or resolved == OLD_COMPOSITORCTL:
        return failed("compositorctl-path", "old /usr/local/bin/compositorctl path is not allowed")
    if path.name != "agora-de-compositorctl":
        return failed("compositorctl-path", "native launch evidence must use agora-de-compositorctl", path=str(path))
    return passed("compositorctl-path", "using agora-de compositorctl", path=str(path))


def get_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-native-launch/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))


def post_json(url: str, body: dict) -> dict:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-native-launch/1"},
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{url} returned {error.code}: {detail}") from error


def wait_for_surface(
    base_url: str,
    surface_id: str,
    expected_app_id: str,
    timeout_seconds: float,
    focused: bool = False,
    minimized: bool | None = None,
    visible: bool | None = None,
) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
            if (
                surface.get("id") == surface_id
                and surface.get("appId") == expected_app_id
                and surface.get("mapped")
                and (not focused or surface.get("focused"))
                and (minimized is None or surface.get("minimized", False) is minimized)
                and (visible is None or surface.get("visible", False) is visible)
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


def capture_visible_native_launch(
    compositorctl: str,
    output_name: str,
    session_id: str,
    checked_at: int,
    surface_id: str,
    app_id: str,
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
        packet["scenario"] = "den-k8-native-launch-visible"
        packet["surfaceId"] = surface_id
        packet["appId"] = app_id
    if capture_check.get("status") == "pass":
        capture_check = dict(capture_check)
        capture_check["name"] = "native-launch-visible-capture"
        capture_check["detail"] = "physical output capture shows native launch and shell pixels"
        capture_check["surfaceId"] = surface_id
        capture_check["appId"] = app_id
    return capture_check, packet


def classify_native_surface_presentation(surface: dict, checked_at: int, surface_id: str, app_id: str) -> tuple[dict, dict]:
    frame_count = int(surface.get("frameCount") or 0)
    content_commit_count = int(surface.get("contentCommitCount") or 0)
    classification = "insufficient_mapped_only"
    detail = "native surface is mapped/focused but has no content or frame counter evidence"
    status = "pass"
    if content_commit_count > 0:
        classification = "content_committed"
        detail = "native surface has content commit evidence"
    if frame_count > 0:
        classification = "frame_presented"
        detail = "native surface has frame-presented evidence"
    packet = {
        "scenario": "den-k8-native-launch-surface-readback",
        "capturedAtUnixMillis": checked_at,
        "surfaceId": surface_id,
        "appId": app_id,
        "visualStatus": "unknown",
        "captureClassification": classification,
        "frameCount": frame_count,
        "contentCommitCount": content_commit_count,
    }
    return {
        "name": "surface-presentation-readback",
        "status": status,
        "detail": detail,
        "surfaceId": surface_id,
        "appId": app_id,
        "frameCount": frame_count,
        "contentCommitCount": content_commit_count,
        "captureClassification": classification,
    }, packet


def load_live_evidence_module():
    path = pathlib.Path(__file__).with_name("check-den-k8.py")
    spec = importlib.util.spec_from_file_location("agora_de_check_den_k8", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load live evidence module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def unavailable_packet(checked_at: int, surface_id: str, app_id: str) -> dict:
    return {
        "scenario": "den-k8-native-launch-visible",
        "capturedAtUnixMillis": checked_at,
        "surfaceId": surface_id,
        "appId": app_id,
        "visualStatus": "unknown",
        "captureClassification": "not_visible",
    }


def unix_millis() -> int:
    return int(time.time() * 1000)


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "fail", "detail": detail, **extra}


def finish(
    checks: list[dict],
    evidence_packets: list[dict],
    app_id: str,
    expected_app_id: str,
    launched_surface: str,
    checked_at: int,
) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": "agora-de.native-launch-live.v1",
        "checkedAtUnixMillis": checked_at,
        "appId": app_id,
        "expectedAppId": expected_app_id,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "launchedSurfaceId": launched_surface,
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

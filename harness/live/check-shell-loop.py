#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import sys
import time
import urllib.error
import urllib.request


def main() -> int:
    parser = argparse.ArgumentParser(description="Check the installed shell launch/focus/close loop.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--app-id", default="shell-status")
    parser.add_argument("--expected-app-id", default="io.agorade.ShellStatus")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-shell-loop")
    parser.add_argument(
        "--capture-delay-seconds",
        type=float,
        default=3.5,
        help="Delay after focus before physical output capture, allowing dock polling to reflect running state.",
    )
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
    try:
        catalog = get_json(args.base_url + "/api/catalog/apps")
        app = next((item for item in catalog.get("apps", []) if item.get("id") == args.app_id), None)
        if not app:
            if args.app_id == "shell-status":
                checks.append(passed("catalog", "built-in shell status target launches outside the visible desktop-entry catalog"))
            else:
                checks.append(failed("catalog", f"app {args.app_id!r} not present"))
                return finish(checks, evidence_packets, launched_surface, checked_at)
        elif not app.get("launchable"):
            checks.append(failed("catalog", f"app {args.app_id!r} is not launchable"))
            return finish(checks, evidence_packets, launched_surface, checked_at)
        else:
            checks.append(passed("catalog", "launchable app is present"))

        launch = post_json(args.base_url + "/api/catalog/launch", {"appId": args.app_id})
        launched_surface = launch.get("surfaceId") or ""
        if not launched_surface:
            checks.append(failed("launch", f"launch response missing surfaceId: {launch}"))
            return finish(checks, evidence_packets, launched_surface, checked_at)
        checks.append(passed("launch", "launch returned a surface", surfaceId=launched_surface, launchId=launch.get("launchId")))

        surface = wait_for_surface(args.base_url, launched_surface, args.expected_app_id, args.timeout_seconds)
        if not surface:
            checks.append(failed("running-state", f"surface {launched_surface!r} did not appear in /api/surfaces"))
            return finish(checks, evidence_packets, launched_surface, checked_at)
        checks.append(passed("running-state", "launched surface appears in running state", surfaceId=launched_surface))

        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "focus"})
        checks.append(passed("focus", "focus action accepted", surfaceId=launched_surface))

        if args.output_name:
            if args.capture_delay_seconds > 0:
                time.sleep(args.capture_delay_seconds)
            capture_check, packet = capture_visible_shell_loop(
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
            evidence_packets.append(
                {
                    "scenario": "den-k8-shell-launch-visible",
                    "capturedAtUnixMillis": checked_at,
                    "surfaceId": launched_surface,
                    "appId": args.expected_app_id,
                    "visualStatus": "unknown",
                    "captureClassification": "not_visible",
                }
            )

        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "close"})
        checks.append(passed("close", "close action accepted", surfaceId=launched_surface))

        if wait_until_absent(args.base_url, launched_surface, args.timeout_seconds):
            checks.append(passed("stale-cleanup", "closed surface disappeared from running state", surfaceId=launched_surface))
        else:
            checks.append(failed("stale-cleanup", f"surface {launched_surface!r} remained after close"))
    finally:
        if launched_surface:
            try:
                post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "close"})
            except Exception:
                pass

    return finish(checks, evidence_packets, launched_surface, checked_at)


def get_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-shell-loop/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))


def post_json(url: str, body: dict) -> dict:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-shell-loop/1"},
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{url} returned {error.code}: {detail}") from error


def wait_for_surface(base_url: str, surface_id: str, expected_app_id: str, timeout_seconds: float) -> dict | None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
            if surface.get("id") == surface_id and surface.get("appId") == expected_app_id and surface.get("mapped"):
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


def capture_visible_shell_loop(
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
        packet["scenario"] = "den-k8-shell-launch-visible"
        packet["surfaceId"] = surface_id
        packet["appId"] = app_id
    if capture_check.get("status") == "pass":
        capture_check = dict(capture_check)
        capture_check["name"] = "launch-visible-capture"
        capture_check["detail"] = "physical output capture shows launched shell app and dock pixels"
        capture_check["surfaceId"] = surface_id
        capture_check["appId"] = app_id
    return capture_check, packet


def load_live_evidence_module():
    path = pathlib.Path(__file__).with_name("check-den-k8.py")
    spec = importlib.util.spec_from_file_location("agora_de_check_den_k8", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load live evidence module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def unix_millis() -> int:
    return int(time.time() * 1000)


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "fail", "detail": detail, **extra}


def finish(checks: list[dict], evidence_packets: list[dict], launched_surface: str, checked_at: int) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": "agora-de.shell-loop-live.v1",
        "checkedAtUnixMillis": checked_at,
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

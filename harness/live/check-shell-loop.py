#!/usr/bin/env python3
import argparse
import json
import sys
import time
import urllib.error
import urllib.request


def main() -> int:
    parser = argparse.ArgumentParser(description="Check the installed shell launch/focus/close loop.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--app-id", default="example-browser")
    parser.add_argument("--expected-app-id", default="io.agorade.ExampleBrowser")
    parser.add_argument("--timeout-seconds", type=float, default=8)
    args = parser.parse_args()

    checks = []
    launched_surface = ""
    try:
        catalog = get_json(args.base_url + "/api/catalog/apps")
        app = next((item for item in catalog.get("apps", []) if item.get("id") == args.app_id), None)
        if not app:
            checks.append(failed("catalog", f"app {args.app_id!r} not present"))
            return finish(checks, launched_surface)
        if not app.get("launchable"):
            checks.append(failed("catalog", f"app {args.app_id!r} is not launchable"))
            return finish(checks, launched_surface)
        checks.append(passed("catalog", "launchable app is present"))

        launch = post_json(args.base_url + "/api/catalog/launch", {"appId": args.app_id})
        launched_surface = launch.get("surfaceId") or ""
        if not launched_surface:
            checks.append(failed("launch", f"launch response missing surfaceId: {launch}"))
            return finish(checks, launched_surface)
        checks.append(passed("launch", "launch returned a surface", surfaceId=launched_surface, launchId=launch.get("launchId")))

        surface = wait_for_surface(args.base_url, launched_surface, args.expected_app_id, args.timeout_seconds)
        if not surface:
            checks.append(failed("running-state", f"surface {launched_surface!r} did not appear in /api/surfaces"))
            return finish(checks, launched_surface)
        checks.append(passed("running-state", "launched surface appears in running state", surfaceId=launched_surface))

        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": launched_surface, "action": "focus"})
        checks.append(passed("focus", "focus action accepted", surfaceId=launched_surface))

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

    return finish(checks, launched_surface)


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


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "fail", "detail": detail, **extra}


def finish(checks: list[dict], launched_surface: str) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": "agora-de.shell-loop-live.v1",
        "checks": checks,
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

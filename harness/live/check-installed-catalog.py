#!/usr/bin/env python3
import argparse
import json
import sys
import time
import urllib.error
import urllib.request


def main() -> int:
    parser = argparse.ArgumentParser(description="Check an installed desktop-entry app catalog route.")
    parser.add_argument("--catalog-url", default="http://127.0.0.1:17782/api/catalog/apps")
    parser.add_argument("--min-apps", type=int, default=1)
    parser.add_argument("--expect-app-id", action="append", default=[])
    parser.add_argument("--forbid-app-id", action="append", default=[])
    parser.add_argument(
        "--require-nonlaunchable",
        action="store_true",
        help="Require at least one visible app with launchable=false.",
    )
    parser.add_argument(
        "--require-all-nonlaunchable",
        action="store_true",
        help="Require every visible app to be non-launchable under the current native-launch policy.",
    )
    parser.add_argument("--timeout-seconds", type=float, default=5)
    args = parser.parse_args()

    checked_at = unix_millis()
    checks = []
    try:
        payload = get_json(args.catalog_url, args.timeout_seconds)
    except Exception as error:
        checks.append(failed("catalog-route", f"catalog route request failed: {error}"))
        return finish(checks, checked_at, 0, [])

    apps = payload.get("apps") if isinstance(payload, dict) else None
    if not isinstance(apps, list):
        checks.append(failed("catalog-shape", "catalog route response must contain apps array"))
        return finish(checks, checked_at, 0, [])
    checks.append(passed("catalog-shape", "catalog route returned apps array", count=len(apps)))

    malformed = first_malformed_app(apps)
    if malformed:
        checks.append(failed("entry-shape", malformed))
        return finish(checks, checked_at, len(apps), apps)
    checks.append(passed("entry-shape", "catalog entries have stable shell fields"))

    if len(apps) < args.min_apps:
        checks.append(failed("entry-count", f"catalog returned {len(apps)} apps, want at least {args.min_apps}"))
    else:
        checks.append(passed("entry-count", "catalog returned installed app entries", count=len(apps)))

    ids = {app["id"] for app in apps}
    for app_id in args.expect_app_id:
        if app_id in ids:
            checks.append(passed("expected-app", f"catalog contains {app_id}", appId=app_id))
        else:
            checks.append(failed("expected-app", f"catalog missing {app_id}", appId=app_id))

    for app_id in args.forbid_app_id:
        if app_id in ids:
            checks.append(failed("forbidden-app", f"catalog unexpectedly contains {app_id}", appId=app_id))
        else:
            checks.append(passed("forbidden-app", f"catalog omits {app_id}", appId=app_id))

    launchable = [app for app in apps if app.get("launchable") is True]
    nonlaunchable = [app for app in apps if app.get("launchable") is not True]
    if args.require_nonlaunchable:
        if nonlaunchable:
            checks.append(passed("nonlaunchable-policy", "catalog includes non-launchable installed entries"))
        else:
            checks.append(failed("nonlaunchable-policy", "catalog has no non-launchable installed entries"))
    if args.require_all_nonlaunchable:
        if launchable:
            sample = ", ".join(app["id"] for app in launchable[:5])
            checks.append(failed("native-launch-policy", f"unexpected launchable installed entries: {sample}"))
        else:
            checks.append(passed("native-launch-policy", "all installed entries are non-launchable"))

    return finish(checks, checked_at, len(apps), apps)


def get_json(url: str, timeout_seconds: float) -> dict:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-installed-catalog/1"})
    try:
        with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{url} returned {error.code}: {detail}") from error


def first_malformed_app(apps: list[object]) -> str | None:
    for index, app in enumerate(apps):
        if not isinstance(app, dict):
            return f"catalog entry {index} must be an object"
        for field in ("id", "name", "icon"):
            if not isinstance(app.get(field), str):
                return f"catalog entry {index} missing string {field}"
        if "launchable" in app and not isinstance(app["launchable"], bool):
            return f"catalog entry {index} launchable must be boolean when present"
    return None


def unix_millis() -> int:
    return int(time.time() * 1000)


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "fail", "detail": detail, **extra}


def finish(checks: list[dict], checked_at: int, app_count: int, apps: list[object]) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    sample = []
    for app in apps[:10]:
        if isinstance(app, dict):
            sample.append(
                {
                    "id": app.get("id"),
                    "name": app.get("name"),
                    "launchable": app.get("launchable") is True,
                }
            )
    result = {
        "schema": "agora-de.installed-catalog-live.v1",
        "checkedAtUnixMillis": checked_at,
        "appCount": app_count,
        "sampleApps": sample,
        "checks": checks,
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

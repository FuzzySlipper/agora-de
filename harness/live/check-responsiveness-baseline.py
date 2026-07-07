#!/usr/bin/env python3
import argparse
import json
import math
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request


SCHEMA = "agora-de.responsiveness-baseline-live.v1"
SHELL_POPUP_APP_IDS = {"io.agorade.ShellStatus", "io.agorade.ShellLauncher"}
SHELL_LAUNCH_IDS = {
    "shell-status": "io.agorade.ShellStatus",
    "shell-launcher": "io.agorade.ShellLauncher",
}
SUCCESSFUL_LAUNCH_STATUSES = {"launched", "surface_observed_after_timeout", "reused_existing_window"}


def main() -> int:
    parser = argparse.ArgumentParser(description="Measure installed-service Agora DE responsiveness baseline.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--app-id", default="Alacritty.desktop")
    parser.add_argument("--expected-app-id", default="Alacritty")
    parser.add_argument("--samples", type=int, default=5)
    parser.add_argument("--timeout-seconds", type=float, default=8.0)
    args = parser.parse_args()

    checked_at = unix_millis()
    checks: list[dict] = []
    measurements: list[dict] = []
    opened_surfaces: set[str] = set()

    path_check = check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checked_at, args, checks, measurements)

    try:
        close_shell_popups(args.base_url)
        checks.append(passed("preflight-cleanup", "closed any stale shell popups before responsiveness probe"))

        for route in [
            "/api/theme",
            "/api/catalog/apps",
            "/api/surfaces",
            "/api/layout",
            "/api/workspaces",
            "/api/operator/status",
        ]:
            measurement, check = measure_get_route(args.base_url, route, max(1, args.samples))
            measurements.append(measurement)
            checks.append(check)

        workspace_measurement, workspace_check = measure_workspace_activation(args)
        measurements.append(workspace_measurement)
        checks.append(workspace_check)

        for shell_launch_id, expected_app_id in SHELL_LAUNCH_IDS.items():
            shell_measurements, shell_checks, opened = measure_shell_popup_cycle(args, shell_launch_id, expected_app_id)
            measurements.extend(shell_measurements)
            checks.extend(shell_checks)
            opened_surfaces.update(opened)

        native_measurements, native_checks, opened = measure_native_app_cycle(args)
        measurements.extend(native_measurements)
        checks.extend(native_checks)
        opened_surfaces.update(opened)
    except Exception as error:
        checks.append(failed("responsiveness-baseline", f"responsiveness probe failed: {error}"))
    finally:
        for surface_id in sorted(opened_surfaces):
            try:
                post_json(args.base_url + "/api/surfaces/action", {"surfaceId": surface_id, "action": "close"})
            except Exception:
                pass
        try:
            close_shell_popups(args.base_url)
        except Exception:
            pass

    cleanup_measurement, cleanup_check = measure_cleanup_state(args)
    measurements.append(cleanup_measurement)
    checks.append(cleanup_check)
    return finish(checked_at, args, checks, measurements)


def measure_get_route(base_url: str, route: str, samples: int) -> tuple[dict, dict]:
    sample_ms = []
    errors = []
    for _ in range(samples):
        started = time.perf_counter()
        try:
            payload = get_json(base_url + route)
            sample_ms.append(elapsed_ms(started))
            if not isinstance(payload, dict):
                errors.append("response was not a JSON object")
        except Exception as error:
            sample_ms.append(elapsed_ms(started))
            errors.append(str(error))
        time.sleep(0.05)
    measurement = timing_measurement(f"route:{route}", "route", sample_ms, route=route, method="GET")
    if errors:
        return measurement, failed(f"route:{route}", f"{route} failed during baseline samples", errors=errors[:3], **timing_fields(sample_ms))
    return measurement, passed(f"route:{route}", f"{route} returned JSON for baseline samples", **timing_fields(sample_ms))


def measure_workspace_activation(args: argparse.Namespace) -> tuple[dict, dict]:
    route = "/api/workspaces/action"
    try:
        workspaces = get_json(args.base_url + "/api/workspaces").get("workspaces") or []
        active = next((item for item in workspaces if item.get("active") and item.get("id")), None)
        workspace_id = (active or workspaces[0]).get("id") if workspaces else "workspace-1"
    except Exception:
        workspace_id = "workspace-1"

    started = time.perf_counter()
    try:
        post_json(args.base_url + route, {"workspaceId": workspace_id, "action": "activate"})
        sample_ms = [elapsed_ms(started)]
    except Exception as error:
        sample_ms = [elapsed_ms(started)]
        return (
            timing_measurement("workspace-activate", "action", sample_ms, route=route, method="POST", workspaceId=workspace_id),
            failed("workspace-activate", f"workspace activation failed: {error}", workspaceId=workspace_id, **timing_fields(sample_ms)),
        )
    return (
        timing_measurement("workspace-activate", "action", sample_ms, route=route, method="POST", workspaceId=workspace_id),
        passed("workspace-activate", "workspace activation action accepted", workspaceId=workspace_id, **timing_fields(sample_ms)),
    )


def measure_shell_popup_cycle(args: argparse.Namespace, shell_launch_id: str, expected_app_id: str) -> tuple[list[dict], list[dict], set[str]]:
    measurements = []
    checks = []
    opened: set[str] = set()

    launch_route = "/api/catalog/launch"
    started = time.perf_counter()
    try:
        launch = post_json(args.base_url + launch_route, {"appId": shell_launch_id})
        launch_ms = elapsed_ms(started)
    except Exception as error:
        launch_ms = elapsed_ms(started)
        samples = [launch_ms]
        measurements.append(timing_measurement(f"{shell_launch_id}:launch-http", "action", samples, route=launch_route, method="POST"))
        checks.append(failed(f"{shell_launch_id}:launch-http", f"{shell_launch_id} launch failed: {error}", **timing_fields(samples)))
        return measurements, checks, opened

    measurements.append(timing_measurement(f"{shell_launch_id}:launch-http", "action", [launch_ms], route=launch_route, method="POST"))
    checks.append(passed(f"{shell_launch_id}:launch-http", f"{shell_launch_id} launch action accepted", **timing_fields([launch_ms])))

    observed, observed_ms = wait_for_app_surface(args.base_url, expected_app_id, args.timeout_seconds)
    measurements.append(
        timing_measurement(
            f"{shell_launch_id}:launch-to-observed",
            "action",
            [observed_ms],
            route="/api/surfaces",
            method="GET",
            appId=expected_app_id,
            phase="launch-to-observed",
        )
    )
    if not observed:
        checks.append(failed(f"{shell_launch_id}:launch-to-observed", f"{expected_app_id} surface was not observed", **timing_fields([observed_ms])))
        return measurements, checks, opened
    surface_id = observed.get("id") or ""
    if surface_id:
        opened.add(surface_id)
    checks.append(
        passed(
            f"{shell_launch_id}:launch-to-observed",
            f"{expected_app_id} surface appeared after launch",
            surfaceId=surface_id,
            **timing_fields([observed_ms]),
        )
    )

    close_measurement, close_check = measure_close_surface(args, surface_id, f"{shell_launch_id}:close")
    measurements.append(close_measurement)
    checks.append(close_check)
    return measurements, checks, opened


def measure_native_app_cycle(args: argparse.Namespace) -> tuple[list[dict], list[dict], set[str]]:
    measurements = []
    checks = []
    opened: set[str] = set()

    try:
        catalog_measurement, catalog_check = measure_get_route(args.base_url, "/api/catalog/apps", 1)
        measurements.append(rename_measurement(catalog_measurement, "native-catalog-check"))
        checks.append(rename_check(catalog_check, "native-catalog-check"))
        catalog = get_json(args.base_url + "/api/catalog/apps")
        apps = catalog.get("apps") or []
    except Exception as error:
        measurements.append(timing_measurement("native-catalog-check", "route", [], route="/api/catalog/apps", method="GET"))
        checks.append(failed("native-catalog-check", f"catalog check failed: {error}"))
        return measurements, checks, opened

    app = next((item for item in apps if isinstance(item, dict) and item.get("id") == args.app_id), None)
    if not app:
        checks.append(skipped("native-launch", f"{args.app_id} is not present in catalog"))
        return measurements, checks, opened
    if app.get("launchable") is not True:
        checks.append(
            skipped(
                "native-launch",
                f"{args.app_id} is not launchable",
                disabledCode=app.get("disabledCode") or "",
                disabledReason=app.get("disabledReason") or "",
            )
        )
        return measurements, checks, opened
    checks.append(passed("native-catalog", f"{args.app_id} is launchable for responsiveness baseline", appId=args.app_id))

    before_surface_ids = known_surface_ids(args.base_url)
    started = time.perf_counter()
    try:
        launch = post_json(args.base_url + "/api/catalog/launch", {"appId": args.app_id})
        launch_ms = elapsed_ms(started)
    except Exception as error:
        launch_ms = elapsed_ms(started)
        measurements.append(timing_measurement("native-launch-http", "action", [launch_ms], route="/api/catalog/launch", method="POST", appId=args.app_id))
        checks.append(failed("native-launch-http", f"native launch failed: {error}", **timing_fields([launch_ms])))
        return measurements, checks, opened

    measurements.append(timing_measurement("native-launch-http", "action", [launch_ms], route="/api/catalog/launch", method="POST", appId=args.app_id))
    status = launch.get("status") or ""
    surface_id = launch.get("surfaceId") or ""
    if status not in SUCCESSFUL_LAUNCH_STATUSES or not surface_id:
        checks.append(failed("native-launch-http", f"unexpected native launch response: {launch}", **timing_fields([launch_ms])))
        return measurements, checks, opened
    checks.append(passed("native-launch-http", "native launch action returned a mapped surface id", surfaceId=surface_id, launchStatus=status, **timing_fields([launch_ms])))

    observed, observed_ms = wait_for_surface(args.base_url, surface_id, args.expected_app_id, args.timeout_seconds)
    measurements.append(
        timing_measurement(
            "native-launch-to-observed",
            "action",
            [observed_ms],
            route="/api/surfaces",
            method="GET",
            appId=args.expected_app_id,
            phase="launch-to-observed",
        )
    )
    if not observed:
        checks.append(failed("native-launch-to-observed", f"surface {surface_id!r} was not observed", surfaceId=surface_id, **timing_fields([observed_ms])))
        return measurements, checks, opened
    checks.append(passed("native-launch-to-observed", "native surface appeared in shell state", surfaceId=surface_id, **timing_fields([observed_ms])))
    if surface_id not in before_surface_ids:
        opened.add(surface_id)

    focus_measurement, focus_check = measure_focus_surface(args, surface_id, args.expected_app_id)
    measurements.append(focus_measurement)
    checks.append(focus_check)

    if surface_id in opened:
        close_measurement, close_check = measure_close_surface(args, surface_id, "native-close")
        measurements.append(close_measurement)
        checks.append(close_check)
    else:
        checks.append(skipped("native-close", "native launch reused an existing surface; close skipped to preserve user state", surfaceId=surface_id))

    return measurements, checks, opened


def measure_focus_surface(args: argparse.Namespace, surface_id: str, expected_app_id: str) -> tuple[dict, dict]:
    started = time.perf_counter()
    try:
        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": surface_id, "action": "focus"})
        focused, observed_ms = wait_for_surface(args.base_url, surface_id, expected_app_id, args.timeout_seconds, focused=True)
        total_ms = elapsed_ms(started)
    except Exception as error:
        total_ms = elapsed_ms(started)
        return (
            timing_measurement("native-focus", "action", [total_ms], route="/api/surfaces/action", method="POST", surfaceId=surface_id),
            failed("native-focus", f"focus action failed: {error}", surfaceId=surface_id, **timing_fields([total_ms])),
        )
    measurement = timing_measurement("native-focus", "action", [total_ms], route="/api/surfaces/action", method="POST", surfaceId=surface_id, observedMs=observed_ms)
    if not focused:
        return measurement, failed("native-focus", f"surface {surface_id!r} did not become focused", surfaceId=surface_id, **timing_fields([total_ms]))
    return measurement, passed("native-focus", "native surface focus action reached focused state", surfaceId=surface_id, observedMs=observed_ms, **timing_fields([total_ms]))


def measure_close_surface(args: argparse.Namespace, surface_id: str, name: str) -> tuple[dict, dict]:
    started = time.perf_counter()
    try:
        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": surface_id, "action": "close"})
        absent, observed_ms = wait_until_absent(args.base_url, surface_id, args.timeout_seconds)
        total_ms = elapsed_ms(started)
    except Exception as error:
        total_ms = elapsed_ms(started)
        return (
            timing_measurement(name, "cleanup", [total_ms], route="/api/surfaces/action", method="POST", surfaceId=surface_id),
            failed(name, f"close action failed: {error}", surfaceId=surface_id, **timing_fields([total_ms])),
        )
    measurement = timing_measurement(name, "cleanup", [total_ms], route="/api/surfaces/action", method="POST", surfaceId=surface_id, observedMs=observed_ms)
    if not absent:
        return measurement, failed(name, f"surface {surface_id!r} remained after close", surfaceId=surface_id, observedMs=observed_ms, **timing_fields([total_ms]))
    return measurement, passed(name, "surface close action removed the surface from shell state", surfaceId=surface_id, observedMs=observed_ms, **timing_fields([total_ms]))


def measure_cleanup_state(args: argparse.Namespace) -> tuple[dict, dict]:
    started = time.perf_counter()
    try:
        surfaces = get_json(args.base_url + "/api/surfaces").get("surfaces") or []
        sample_ms = [elapsed_ms(started)]
    except Exception as error:
        sample_ms = [elapsed_ms(started)]
        return (
            timing_measurement("cleanup", "cleanup", sample_ms, route="/api/surfaces", method="GET"),
            failed("cleanup", f"cleanup verification failed: {error}", **timing_fields(sample_ms)),
        )
    remaining = [surface for surface in surfaces if surface.get("appId") in SHELL_POPUP_APP_IDS]
    measurement = timing_measurement("cleanup", "cleanup", sample_ms, route="/api/surfaces", method="GET")
    if remaining:
        return measurement, failed("cleanup", "shell popup surfaces remained after responsiveness probe", remaining=remaining, **timing_fields(sample_ms))
    return measurement, passed("cleanup", "responsiveness probe left no shell popup surfaces", **timing_fields(sample_ms))


def check_compositorctl_path(compositorctl: str) -> dict:
    path = pathlib.Path(compositorctl)
    if path.name != "agora-de-compositorctl":
        return failed("compositorctl", "responsiveness evidence must use agora-de-compositorctl", path=str(path))
    try:
        completed = subprocess.run([compositorctl, "--pretty", "list-surfaces"], check=False, text=True, capture_output=True, timeout=3)
    except (OSError, subprocess.TimeoutExpired) as error:
        return failed("compositorctl", f"compositorctl unavailable: {error}", path=str(path))
    if completed.returncode != 0:
        return failed("compositorctl", "compositorctl list-surfaces failed", path=str(path), stderr=completed.stderr.strip())
    try:
        json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        return failed("compositorctl", f"compositorctl list-surfaces returned invalid JSON: {error}", path=str(path))
    return passed("compositorctl", "compositorctl is available for installed-service evidence", path=str(path))


def close_shell_popups(base_url: str) -> None:
    surfaces = get_json(base_url + "/api/surfaces").get("surfaces") or []
    for surface in surfaces:
        if surface.get("appId") in SHELL_POPUP_APP_IDS and surface.get("id"):
            try:
                post_json(base_url + "/api/surfaces/action", {"surfaceId": surface["id"], "action": "close"})
            except Exception:
                pass


def known_surface_ids(base_url: str) -> set[str]:
    try:
        return {
            surface.get("id")
            for surface in get_json(base_url + "/api/surfaces").get("surfaces", [])
            if surface.get("id")
        }
    except Exception:
        return set()


def wait_for_app_surface(base_url: str, expected_app_id: str, timeout_seconds: float) -> tuple[dict | None, float]:
    started = time.perf_counter()
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
            if surface.get("appId") == expected_app_id and surface.get("mapped"):
                return surface, elapsed_ms(started)
        time.sleep(0.1)
    return None, elapsed_ms(started)


def wait_for_surface(
    base_url: str,
    surface_id: str,
    expected_app_id: str,
    timeout_seconds: float,
    focused: bool = False,
) -> tuple[dict | None, float]:
    started = time.perf_counter()
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
            if (
                surface.get("id") == surface_id
                and surface.get("appId") == expected_app_id
                and surface.get("mapped")
                and (not focused or surface.get("focused"))
            ):
                return surface, elapsed_ms(started)
        time.sleep(0.1)
    return None, elapsed_ms(started)


def wait_until_absent(base_url: str, surface_id: str, timeout_seconds: float) -> tuple[bool, float]:
    started = time.perf_counter()
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        surfaces = get_json(base_url + "/api/surfaces").get("surfaces", [])
        if not any(surface.get("id") == surface_id for surface in surfaces):
            return True, elapsed_ms(started)
        time.sleep(0.1)
    return False, elapsed_ms(started)


def get_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-responsiveness-baseline/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))


def post_json(url: str, body: dict) -> dict:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-responsiveness-baseline/1"},
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{url} returned {error.code}: {detail}") from error


def timing_measurement(name: str, category: str, samples_ms: list[float], **extra: object) -> dict:
    fields = timing_fields(samples_ms)
    return {
        "name": name,
        "category": category,
        "unit": "ms",
        "count": len(samples_ms),
        "samplesMs": [round(value, 3) for value in samples_ms],
        **fields,
        **extra,
    }


def timing_fields(samples_ms: list[float]) -> dict:
    if not samples_ms:
        return {"minMs": None, "p50Ms": None, "p95Ms": None, "maxMs": None}
    ordered = sorted(samples_ms)
    return {
        "minMs": round(ordered[0], 3),
        "p50Ms": round(percentile(ordered, 50), 3),
        "p95Ms": round(percentile(ordered, 95), 3),
        "maxMs": round(ordered[-1], 3),
    }


def percentile(ordered: list[float], percentile_value: int) -> float:
    if len(ordered) == 1:
        return ordered[0]
    rank = (percentile_value / 100) * (len(ordered) - 1)
    lower = math.floor(rank)
    upper = math.ceil(rank)
    if lower == upper:
        return ordered[int(rank)]
    lower_weight = upper - rank
    upper_weight = rank - lower
    return ordered[lower] * lower_weight + ordered[upper] * upper_weight


def rename_measurement(measurement: dict, name: str) -> dict:
    updated = dict(measurement)
    updated["name"] = name
    return updated


def rename_check(check: dict, name: str) -> dict:
    updated = dict(check)
    updated["name"] = name
    return updated


def elapsed_ms(started: float) -> float:
    return (time.perf_counter() - started) * 1000


def unix_millis() -> int:
    return int(time.time() * 1000)


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "responsiveness", "status": "pass", "detail": detail, **extra}


def skipped(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "responsiveness", "status": "skip", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "responsiveness", "status": "fail", "detail": detail, **extra}


def finish(checked_at: int, args: argparse.Namespace, checks: list[dict], measurements: list[dict]) -> int:
    failed_checks = [check for check in checks if check.get("status") == "fail"]
    skipped_checks = [check for check in checks if check.get("status") == "skip"]
    result = {
        "schema": SCHEMA,
        "checkedAtUnixMillis": checked_at,
        "baseUrl": args.base_url,
        "appId": args.app_id,
        "expectedAppId": args.expected_app_id,
        "checks": checks,
        "measurements": measurements,
        "summary": {
            "status": "fail" if failed_checks else "pass",
            "passed": len([check for check in checks if check.get("status") == "pass"]),
            "skipped": len(skipped_checks),
            "failed": len(failed_checks),
        },
    }
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 1 if failed_checks else 0


if __name__ == "__main__":
    raise SystemExit(main())

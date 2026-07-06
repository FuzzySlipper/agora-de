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
    parser = argparse.ArgumentParser(description="Check installed-service layout commands through agora-de-compositorctl.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--app-id", action="append", default=[], help="App id to launch; repeat for two or more apps.")
    parser.add_argument("--expected-app-id", action="append", default=[], help="Expected compositor app id for each app.")
    parser.add_argument("--expected-zone", action="append", default=[], help="Expected zone for each app.")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-layout-commands")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.5)
    parser.add_argument("--require-capture", action="store_true")
    parser.add_argument("--timeout-seconds", type=float, default=10)
    args = parser.parse_args()

    structured = load_structured_module()
    app_ids = args.app_id or ["Alacritty.desktop", "foot.desktop"]
    expected_app_ids = args.expected_app_id or ["Alacritty", "foot"]
    expected_zones = args.expected_zone or ["primary", "secondary"]
    checked_at = structured.unix_millis()
    checks = []
    evidence_packets = []
    launched = []
    latest_layout = {}

    if len(app_ids) < 2:
        checks.append(failed("config", "layout command proof requires at least two --app-id values"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
    if len(expected_app_ids) != len(app_ids) or len(expected_zones) != len(app_ids):
        checks.append(failed("config", "expected app/zone counts must match app count"))
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

    path_check = structured.check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

    try:
        catalog = structured.get_json(args.base_url + "/api/catalog/apps")
        apps = catalog.get("apps", [])
        for app_id in app_ids:
            app = next((item for item in apps if isinstance(item, dict) and item.get("id") == app_id), None)
            if not app or app.get("launchable") is not True:
                checks.append(failed("catalog", f"app {app_id!r} is not launchable", appId=app_id))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
        checks.append(passed("catalog", "all layout-command apps are launchable", appIds=app_ids))

        for app_id, expected_app_id in zip(app_ids, expected_app_ids):
            launch = structured.post_json(args.base_url + "/api/catalog/launch", {"appId": app_id})
            surface_id = launch.get("surfaceId") or ""
            if launch.get("status") not in SUCCESSFUL_LAUNCH_STATUSES or not surface_id:
                checks.append(failed("launch", f"unexpected launch response for {app_id!r}: {launch}", appId=app_id))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
            launched.append({"appId": app_id, "expectedAppId": expected_app_id, "surfaceId": surface_id})
            checks.append(passed("launch", "native app launched through shellui", appId=app_id, surfaceId=surface_id))

        for item in launched:
            surface = structured.wait_for_surface(args.base_url, item["surfaceId"], item["expectedAppId"], args.timeout_seconds)
            if not surface:
                checks.append(failed("visibility", f"surface {item['surfaceId']!r} did not become visible"))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
            item["surface"] = surface
        checks.append(passed("visibility", "all launched surfaces are mapped and visible", surfaceIds=surface_ids(launched)))

        for item in launched:
            focus = run_compositorctl_json(args.compositorctl, ["surface", "focus", "--surface", item["surfaceId"], "--timeout-ms", "2000"])
            if focus.get("decision") != "accepted":
                checks.append(failed("focus-command", "focus command was not accepted", surfaceId=item["surfaceId"], response=focus))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
        checks.append(passed("focus-command", "agora-de-compositorctl focus accepted for each surface", surfaceIds=surface_ids(launched)))

        assignments = []
        for item, zone_id in zip(launched, expected_zones):
            response = run_compositorctl_json(
                args.compositorctl,
                ["surface", "assign-zone", "--surface", item["surfaceId"], "--workspace", "workspace-1", "--zone", zone_id, "--timeout-ms", "2000"],
            )
            if response.get("decision") != "accepted":
                checks.append(failed("assign-zone-command", "assign-zone command was not accepted", surfaceId=item["surfaceId"], response=response))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
            assignments.append({"surfaceId": item["surfaceId"], "zoneId": zone_id})
        checks.append(passed("assign-zone-command", "agora-de-compositorctl assigned expected zones", assignments=assignments))

        latest_layout = wait_for_cli_layout(args.compositorctl, launched, expected_zones, args.timeout_seconds)
        if not latest_layout:
            latest_layout = normalize_cli_layout(run_compositorctl_json(args.compositorctl, ["layout", "get"]).get("layout") or {})
            checks.append(failed("layout-command-readback", "layout get did not show expected zones after CLI assignment"))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
        checks.append(passed("layout-command-readback", "layout get reports CLI command results", revision=latest_layout.get("revision", 0)))

        occlusion = structured.check_occlusion_or_zones(latest_layout, launched, expected_zones)
        checks.append(occlusion)
        if occlusion["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

        api_layout = structured.wait_for_expected_zones(args.base_url, launched, expected_zones, args.timeout_seconds) or {}
        if api_layout:
            checks.append(passed("api-layout-observes-command", "/api/layout observes CLI command results"))
        else:
            checks.append(failed("api-layout-observes-command", "/api/layout did not observe CLI command results"))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

        unsupported = expect_backend_unsupported(args.compositorctl, ["layout", "set-mode", "--mode", "columns"])
        checks.append(unsupported)
        if unsupported["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

        workspace_unsupported = expect_backend_unsupported(args.compositorctl, ["workspace", "activate", "--workspace", "workspace-1"])
        checks.append(workspace_unsupported)
        if workspace_unsupported["status"] != "pass":
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

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
            if capture_check.get("status") == "pass":
                capture_check["name"] = "layout-commands-visible-capture"
                capture_check["detail"] = "physical output capture shows CLI layout command result"
            checks.append(capture_check)
            if packet:
                packet = dict(packet)
                packet["scenario"] = "den-k8-layout-commands-visible"
                evidence_packets.append(packet)
        elif args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))
            return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)

        for item in launched:
            close = run_compositorctl_json(args.compositorctl, ["surface", "close", "--surface", item["surfaceId"]])
            if close.get("decision") != "accepted":
                checks.append(failed("close-command", "close command was not accepted", surfaceId=item["surfaceId"], response=close))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
            if structured.wait_until_absent(args.base_url, item["surfaceId"], args.timeout_seconds):
                checks.append(passed("close-command", "CLI close removed surface", surfaceId=item["surfaceId"]))
            else:
                checks.append(failed("close-command", "surface remained after CLI close", surfaceId=item["surfaceId"]))
                return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)
    finally:
        for item in launched:
            try:
                structured.post_json(args.base_url + "/api/surfaces/action", {"surfaceId": item["surfaceId"], "action": "close"})
            except Exception:
                pass

    return finish(checks, evidence_packets, app_ids, expected_app_ids, launched, latest_layout, checked_at)


def load_structured_module():
    path = pathlib.Path(__file__).with_name("check-structured-layout.py")
    spec = importlib.util.spec_from_file_location("agora_de_check_structured_layout", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load structured layout module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def run_compositorctl_json(compositorctl: str, args: list[str]) -> dict:
    result = subprocess.run([compositorctl, "--pretty", *args], check=False, text=True, capture_output=True)
    if result.returncode != 0:
        raise RuntimeError(f"compositorctl {' '.join(args)} failed: {result.stderr.strip() or result.stdout.strip()}")
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise RuntimeError(f"compositorctl {' '.join(args)} returned invalid JSON: {result.stdout}") from error


def expect_backend_unsupported(compositorctl: str, args: list[str]) -> dict:
    result = subprocess.run([compositorctl, "--pretty", *args], check=False, text=True, capture_output=True)
    combined = (result.stderr + "\n" + result.stdout).strip()
    if result.returncode == 0:
        return failed("unsupported-command-class", "unsupported command unexpectedly succeeded", command=args, output=result.stdout)
    if "server[backend_unsupported]" not in combined:
        return failed("unsupported-command-class", "unsupported command lacked backend_unsupported classification", command=args, output=combined)
    return passed("unsupported-command-class", "unsupported command returned backend_unsupported", command=args)


def wait_for_cli_layout(compositorctl: str, launched: list[dict], expected_zones: list[str], timeout_seconds: float) -> dict | None:
    expected = dict(zip(surface_ids(launched), expected_zones))
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        layout = normalize_cli_layout(run_compositorctl_json(compositorctl, ["layout", "get"]).get("layout") or {})
        surfaces = {surface.get("surfaceId"): surface for surface in layout.get("surfaces", [])}
        if all((surfaces.get(surface_id) or {}).get("zoneId") == zone_id for surface_id, zone_id in expected.items()):
            return layout
        time.sleep(0.25)
    return None


def normalize_cli_layout(layout: dict) -> dict:
    return {
        "mode": layout.get("mode", ""),
        "revision": layout.get("revision", 0),
        "surfaces": [normalize_cli_surface(surface) for surface in layout.get("surfaces", [])],
        "workspaces": layout.get("workspaces", []),
    }


def normalize_cli_surface(surface: dict) -> dict:
    return {
        "surfaceId": surface.get("surface_id", ""),
        "label": surface.get("label", ""),
        "appId": surface.get("app_id", ""),
        "title": surface.get("title", ""),
        "role": surface.get("role", ""),
        "outputId": surface.get("output_id", ""),
        "workspaceId": surface.get("workspace_id", ""),
        "zoneId": surface.get("zone_id", ""),
        "mode": surface.get("mode", ""),
        "participation": surface.get("participation", ""),
        "floating": surface.get("floating", False),
        "focused": surface.get("focused", False),
        "visible": surface.get("visible", False),
        "geometry": surface.get("geometry"),
        "order": surface.get("order", 0),
    }


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
    checked_at: int,
) -> int:
    failed_checks = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": "agora-de.layout-commands-live.v1",
        "checkedAtUnixMillis": checked_at,
        "appIds": app_ids,
        "expectedAppIds": expected_app_ids,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "launched": launched,
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

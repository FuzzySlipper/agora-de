#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import shlex
import subprocess
import sys
import time
import urllib.error
import urllib.request


SCHEMA = "agora-de.live-session-soak.v1"
SCENARIO = "den-k8-live-session-soak"
OLD_COMPOSITORCTL = pathlib.Path("/usr/local/bin/compositorctl")
SUCCESSFUL_LAUNCH_STATUSES = {"launched", "surface_observed_after_timeout", "reused_existing_window"}
SHELL_POPUP_TARGETS = {
    "shell-status": "io.agorade.ShellStatus",
    "shell-launcher": "io.agorade.ShellLauncher",
}
SHELL_POPUP_APP_IDS = set(SHELL_POPUP_TARGETS.values())
SHELL_CHROME_APP_IDS = {
    "io.agorade.ShellBackground",
    "io.agorade.ShellOverlay",
    "io.agorade.ShellPanel",
    *SHELL_POPUP_APP_IDS,
}
AGORA_PROCESS_NEEDLES = ("agora-de", "wayfire")


def main() -> int:
    parser = argparse.ArgumentParser(description="Run a bounded installed-session Agora DE soak probe.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--app-id", action="append", default=[], help="Native app id to launch during each cycle.")
    parser.add_argument("--expected-app-id", action="append", default=[], help="Expected compositor app id matching each --app-id.")
    parser.add_argument("--cycles", type=int, default=2)
    parser.add_argument("--cycle-delay-seconds", type=float, default=0.35)
    parser.add_argument("--timeout-seconds", type=float, default=10.0)
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default=SCENARIO)
    parser.add_argument("--capture-delay-seconds", type=float, default=1.0)
    parser.add_argument("--require-capture", action="store_true")
    parser.add_argument("--restart-command", default="", help="Optional explicit command to run once during the soak.")
    parser.add_argument("--restart-timeout-seconds", type=float, default=20.0)
    parser.add_argument("--journal-lines", type=int, default=80)
    parser.add_argument("--artifact-dir", default="", help="Optional directory for summary, samples, journals, and capture packets.")
    args = parser.parse_args()

    app_ids = args.app_id or ["Alacritty.desktop"]
    expected_app_ids = args.expected_app_id or ["Alacritty"]
    checked_at = unix_millis()
    checks: list[dict] = []
    samples: list[dict] = []
    events: list[dict] = []
    evidence_packets: list[dict] = []
    opened_surfaces: set[str] = set()

    if args.cycles < 1:
        checks.append(failed("config", "--cycles must be at least 1"))
        return finish(args, checked_at, app_ids, expected_app_ids, checks, samples, events, evidence_packets)
    if len(app_ids) != len(expected_app_ids):
        checks.append(failed("config", "--expected-app-id count must match --app-id count"))
        return finish(args, checked_at, app_ids, expected_app_ids, checks, samples, events, evidence_packets)

    checks.append(check_compositorctl_path(args.compositorctl))
    if checks[-1]["status"] == "fail":
        return finish(args, checked_at, app_ids, expected_app_ids, checks, samples, events, evidence_packets)

    try:
        route_check = check_routes(args.base_url)
        checks.append(route_check)
        if route_check["status"] == "fail":
            return finish(args, checked_at, app_ids, expected_app_ids, checks, samples, events, evidence_packets)

        catalog_check = check_catalog(args.base_url, app_ids)
        checks.append(catalog_check)
        if catalog_check["status"] == "fail":
            return finish(args, checked_at, app_ids, expected_app_ids, checks, samples, events, evidence_packets)

        close_shell_popups(args.base_url)
        samples.append(sample_state(args, "initial"))

        overlay_check = check_overlay_health(args.base_url)
        checks.append(overlay_check)
        if overlay_check["status"] == "fail":
            return finish(args, checked_at, app_ids, expected_app_ids, checks, samples, events, evidence_packets)

        for cycle in range(1, args.cycles + 1):
            cycle_events: list[dict] = []
            workspace_check, workspace_events = exercise_workspaces(args)
            checks.append(named_cycle_check(workspace_check, cycle))
            cycle_events.extend(workspace_events)

            popup_check, popup_events, opened = exercise_shell_popups(args)
            checks.append(named_cycle_check(popup_check, cycle))
            cycle_events.extend(popup_events)
            opened_surfaces.update(opened)

            native_check, native_events, opened = exercise_native_apps(args, app_ids, expected_app_ids)
            checks.append(named_cycle_check(native_check, cycle))
            cycle_events.extend(native_events)
            opened_surfaces.update(opened)

            drift_check = check_surface_layout_drift(args.base_url)
            checks.append(named_cycle_check(drift_check, cycle))

            events.append({"cycle": cycle, "events": cycle_events})
            samples.append(sample_state(args, f"cycle-{cycle}"))
            if args.cycle_delay_seconds > 0:
                time.sleep(args.cycle_delay_seconds)

        if args.restart_command:
            restart_check, restart_events = exercise_restart_probe(args)
            checks.append(restart_check)
            events.append({"cycle": "restart", "events": restart_events})
            samples.append(sample_state(args, "after-restart"))

        if args.output_name:
            capture_check, packet = capture_output(args, checked_at, active_work_surface_ids(args.base_url))
            checks.append(capture_check)
            if packet:
                evidence_packets.append(packet)
        elif args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))

        cleanup_check = cleanup_soak(args, opened_surfaces)
        checks.append(cleanup_check)
        samples.append(sample_state(args, "final"))
    except Exception as error:
        checks.append(failed("soak", f"live-session soak failed: {error}"))
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

    return finish(args, checked_at, app_ids, expected_app_ids, checks, samples, events, evidence_packets)


def check_routes(base_url: str) -> dict:
    routes = ["/api/catalog/apps", "/api/surfaces", "/api/layout", "/api/workspaces", "/api/operator/status"]
    failures = []
    for route in routes:
        try:
            payload = get_json(base_url + route)
            if not isinstance(payload, dict):
                failures.append({"route": route, "reason": "response was not a JSON object"})
        except Exception as error:
            failures.append({"route": route, "reason": str(error)})
    try:
        body = get_text(base_url + "/shell/dist/desktop/?surface=overlay")
        if "agent-overlay-surface" not in body:
            failures.append({"route": "/shell/dist/desktop/?surface=overlay", "reason": "overlay route missing surface hook"})
    except Exception as error:
        failures.append({"route": "/shell/dist/desktop/?surface=overlay", "reason": str(error)})
    if failures:
        return failed("route-health", "one or more installed routes failed during soak preflight", failures=failures)
    return passed("route-health", "installed shell routes are available for live-session soak")


def check_catalog(base_url: str, app_ids: list[str]) -> dict:
    catalog = get_json(base_url + "/api/catalog/apps").get("apps") or []
    failures = []
    for app_id in app_ids:
        app = next((item for item in catalog if isinstance(item, dict) and item.get("id") == app_id), None)
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
        return failed("catalog", "one or more soak launch targets are unavailable", failures=failures)
    return passed("catalog", "native soak launch targets are launchable", appIds=app_ids)


def check_overlay_health(base_url: str) -> dict:
    surfaces = get_json(base_url + "/api/surfaces").get("surfaces") or []
    overlay = next((surface for surface in surfaces if surface.get("appId") == "io.agorade.ShellOverlay"), None)
    if not overlay:
        return failed("overlay", "installed ShellOverlay surface is not present")
    if overlay.get("mapped") is False or overlay.get("visible") is False:
        return failed("overlay", "installed ShellOverlay surface is not mapped and visible", surface=overlay)
    return passed("overlay", "installed overlay route and surface are healthy", surfaceId=overlay.get("id") or "")


def exercise_workspaces(args: argparse.Namespace) -> tuple[dict, list[dict]]:
    state = get_json(args.base_url + "/api/workspaces")
    workspaces = [item for item in state.get("workspaces", []) if isinstance(item, dict) and item.get("id")]
    if not workspaces:
        return failed("workspace-switch", "workspace route returned no workspaces"), []

    events = []
    targets = workspaces if len(workspaces) > 1 else [workspaces[0]]
    for workspace in targets:
        body = {"workspaceId": workspace["id"], "action": "activate"}
        if workspace.get("outputId"):
            body["outputId"] = workspace["outputId"]
        response = post_json(args.base_url + "/api/workspaces/action", body)
        if response.get("status") != "accepted":
            return failed("workspace-switch", "workspace activation was not accepted", workspaceId=workspace["id"], response=response), events
        events.append({"action": "workspace.activate", "workspaceId": workspace["id"], "outputId": workspace.get("outputId") or ""})
    return passed("workspace-switch", "workspace activation path stayed responsive", count=len(targets)), events


def exercise_shell_popups(args: argparse.Namespace) -> tuple[dict, list[dict], set[str]]:
    events = []
    opened: set[str] = set()
    for launch_id, expected_app_id in SHELL_POPUP_TARGETS.items():
        before = known_surface_ids(args.base_url)
        launch = post_json(args.base_url + "/api/catalog/launch", {"appId": launch_id})
        surface_id = launch.get("surfaceId") or ""
        if launch.get("status") not in SUCCESSFUL_LAUNCH_STATUSES or not surface_id:
            return failed("shell-popup", f"unexpected launch response for {launch_id}", response=launch), events, opened
        surface, observed_ms = wait_for_surface(args.base_url, surface_id, expected_app_id, args.timeout_seconds)
        if not surface:
            return failed("shell-popup", f"{expected_app_id} did not appear after launch", surfaceId=surface_id, observedMs=observed_ms), events, opened
        if surface_id not in before:
            opened.add(surface_id)
        events.append({"action": "popup.launch", "appId": launch_id, "surfaceId": surface_id, "observedMs": round(observed_ms, 3)})
        post_json(args.base_url + "/api/surfaces/action", {"surfaceId": surface_id, "action": "close"})
        absent, close_ms = wait_until_absent(args.base_url, surface_id, args.timeout_seconds)
        if not absent:
            return failed("shell-popup", f"{expected_app_id} remained after close", surfaceId=surface_id, observedMs=close_ms), events, opened
        opened.discard(surface_id)
        events.append({"action": "popup.close", "appId": launch_id, "surfaceId": surface_id, "observedMs": round(close_ms, 3)})
    return passed("shell-popup", "launcher and status popups opened and closed without stale shell surfaces"), events, opened


def exercise_native_apps(args: argparse.Namespace, app_ids: list[str], expected_app_ids: list[str]) -> tuple[dict, list[dict], set[str]]:
    events = []
    opened: set[str] = set()
    for app_id, expected_app_id in zip(app_ids, expected_app_ids):
        before = known_surface_ids(args.base_url)
        launch = post_json(args.base_url + "/api/catalog/launch", {"appId": app_id})
        surface_id = launch.get("surfaceId") or ""
        if launch.get("status") not in SUCCESSFUL_LAUNCH_STATUSES or not surface_id:
            return failed("native-cycle", f"unexpected launch response for {app_id}", response=launch), events, opened
        if surface_id not in before:
            opened.add(surface_id)
        surface, observed_ms = wait_for_surface(args.base_url, surface_id, expected_app_id, args.timeout_seconds)
        if not surface:
            return failed("native-cycle", f"{expected_app_id} did not appear after launch", surfaceId=surface_id, observedMs=observed_ms), events, opened
        events.append({"action": "native.launch", "appId": app_id, "surfaceId": surface_id, "observedMs": round(observed_ms, 3)})

        for action, body, predicate in [
            ("focus", {"surfaceId": surface_id, "action": "focus"}, lambda item: bool(item.get("focused"))),
            ("minimize", {"surfaceId": surface_id, "action": "minimize", "enabled": True}, lambda item: bool(item.get("minimized"))),
            ("restore", {"surfaceId": surface_id, "action": "minimize", "enabled": False}, lambda item: not bool(item.get("minimized")) and bool(item.get("mapped"))),
        ]:
            response = post_json(args.base_url + "/api/surfaces/action", body)
            if response.get("status") != "accepted":
                return failed("native-cycle", f"{action} action was not accepted", surfaceId=surface_id, response=response), events, opened
            observed, action_ms = wait_for_surface(args.base_url, surface_id, expected_app_id, args.timeout_seconds, predicate=predicate)
            if not observed:
                return failed("native-cycle", f"{action} state was not observed", surfaceId=surface_id, observedMs=action_ms), events, opened
            events.append({"action": f"native.{action}", "surfaceId": surface_id, "observedMs": round(action_ms, 3)})

        if surface_id in opened:
            post_json(args.base_url + "/api/surfaces/action", {"surfaceId": surface_id, "action": "close"})
            absent, close_ms = wait_until_absent(args.base_url, surface_id, args.timeout_seconds)
            if not absent:
                return failed("native-cycle", "launched native surface remained after close", surfaceId=surface_id, observedMs=close_ms), events, opened
            opened.discard(surface_id)
            events.append({"action": "native.close", "surfaceId": surface_id, "observedMs": round(close_ms, 3)})
        else:
            events.append({"action": "native.close.skip", "surfaceId": surface_id, "reason": "reused existing surface"})
    return passed("native-cycle", "native app launch, focus, minimize, restore, and close path stayed stable"), events, opened


def check_surface_layout_drift(base_url: str) -> dict:
    surfaces = get_json(base_url + "/api/surfaces").get("surfaces") or []
    layout = get_json(base_url + "/api/layout").get("layout") or {}
    layout_ids = {
        surface.get("surfaceId")
        for surface in layout.get("surfaces", [])
        if isinstance(surface, dict) and surface.get("surfaceId")
    }
    work_surfaces = [
        surface
        for surface in surfaces
        if surface.get("id")
        and surface.get("appId") not in SHELL_CHROME_APP_IDS
        and surface.get("surfaceKind") != "layer_shell"
        and surface.get("mapped") is not False
    ]
    missing_layout = [surface.get("id") for surface in work_surfaces if surface.get("id") not in layout_ids and surface.get("appId") not in SHELL_POPUP_APP_IDS]
    stale_popups = [surface.get("id") for surface in surfaces if surface.get("appId") in SHELL_POPUP_APP_IDS]
    if missing_layout or stale_popups:
        return failed("state-drift", "surface, layout, or popup state drifted during soak", missingLayout=missing_layout, stalePopups=stale_popups)
    return passed("state-drift", "surface/layout state has no stale popup or missing-layout drift", workSurfaceCount=len(work_surfaces))


def exercise_restart_probe(args: argparse.Namespace) -> tuple[dict, list[dict]]:
    command = shlex.split(args.restart_command)
    if not command:
        return failed("restart", "restart command was empty after parsing"), []
    started = time.perf_counter()
    try:
        completed = subprocess.run(command, check=False, text=True, capture_output=True, timeout=args.restart_timeout_seconds)
    except (OSError, subprocess.TimeoutExpired) as error:
        return failed("restart", f"restart command failed to run: {error}", command=command), []
    elapsed = elapsed_ms(started)
    event = {
        "action": "restart.command",
        "command": command,
        "returnCode": completed.returncode,
        "elapsedMs": round(elapsed, 3),
        "stdoutTail": completed.stdout[-2000:],
        "stderrTail": completed.stderr[-2000:],
    }
    if completed.returncode != 0:
        return failed("restart", "restart command returned non-zero", **event), [event]
    deadline = time.time() + args.timeout_seconds
    while time.time() < deadline:
        try:
            get_json(args.base_url + "/api/operator/status")
            get_json(args.base_url + "/api/surfaces")
            return passed("restart", "explicit restart command completed and shell routes recovered", elapsedMs=round(elapsed, 3)), [event]
        except Exception:
            time.sleep(0.25)
    return failed("restart", "shell routes did not recover after restart command", **event), [event]


def cleanup_soak(args: argparse.Namespace, opened_surfaces: set[str]) -> dict:
    failures = []
    for surface_id in sorted(opened_surfaces):
        try:
            post_json(args.base_url + "/api/surfaces/action", {"surfaceId": surface_id, "action": "close"})
            absent, observed_ms = wait_until_absent(args.base_url, surface_id, args.timeout_seconds)
            if not absent:
                failures.append({"surfaceId": surface_id, "reason": "remained after close", "observedMs": round(observed_ms, 3)})
        except Exception as error:
            failures.append({"surfaceId": surface_id, "reason": str(error)})
    try:
        close_shell_popups(args.base_url)
    except Exception as error:
        failures.append({"surfaceId": "shell-popups", "reason": str(error)})
    drift = check_surface_layout_drift(args.base_url)
    if drift["status"] == "fail":
        failures.append({"surfaceId": "state-drift", "reason": drift["detail"], "drift": drift})
    if failures:
        return failed("cleanup", "soak cleanup left stale surfaces or state drift", failures=failures)
    return passed("cleanup", "soak cleanup left no opened native or shell popup surfaces")


def sample_state(args: argparse.Namespace, phase: str) -> dict:
    sample = {
        "phase": phase,
        "sampledAtUnixMillis": unix_millis(),
        "routes": {},
        "processes": collect_process_sample(),
    }
    for route in ["/api/surfaces", "/api/layout", "/api/workspaces", "/api/operator/status"]:
        try:
            payload = get_json(args.base_url + route)
            sample["routes"][route] = summarize_route(route, payload)
        except Exception as error:
            sample["routes"][route] = {"status": "error", "error": str(error)}
    return sample


def summarize_route(route: str, payload: dict) -> dict:
    if route == "/api/surfaces":
        surfaces = payload.get("surfaces") or []
        return {
            "status": "ok",
            "surfaceCount": len(surfaces),
            "mappedCount": len([surface for surface in surfaces if surface.get("mapped")]),
            "focusedSurfaceIds": [surface.get("id") for surface in surfaces if surface.get("focused") and surface.get("id")],
            "shellPopupSurfaceIds": [surface.get("id") for surface in surfaces if surface.get("appId") in SHELL_POPUP_APP_IDS],
        }
    if route == "/api/layout":
        layout = payload.get("layout") or {}
        return {
            "status": "ok",
            "mode": layout.get("mode") or "",
            "revision": layout.get("revision") or 0,
            "surfaceCount": len(layout.get("surfaces") or []),
            "workspaceCount": len(layout.get("workspaces") or []),
        }
    if route == "/api/workspaces":
        workspaces = payload.get("workspaces") or []
        return {
            "status": "ok",
            "currentWorkspaceId": payload.get("currentWorkspaceId") or "",
            "currentOutputId": payload.get("currentOutputId") or "",
            "workspaceCount": len(workspaces),
            "activeWorkspaceIds": [workspace.get("id") for workspace in workspaces if workspace.get("active") and workspace.get("id")],
        }
    if route == "/api/operator/status":
        return {
            "status": "ok",
            "healthy": payload.get("healthy"),
            "serviceCount": len(payload.get("services") or []),
            "socketCount": len(payload.get("sockets") or []),
            "outputCount": len(payload.get("outputs") or []),
        }
    return {"status": "ok"}


def collect_process_sample() -> list[dict]:
    try:
        completed = subprocess.run(["ps", "-eo", "pid=,comm=,rss=,vsz=,args="], check=False, text=True, capture_output=True, timeout=3)
    except (OSError, subprocess.TimeoutExpired):
        return []
    if completed.returncode != 0:
        return []
    processes = []
    for line in completed.stdout.splitlines():
        parts = line.strip().split(None, 4)
        if len(parts) < 5:
            continue
        pid, command, rss, vsz, args = parts
        haystack = f"{command} {args}"
        if not any(needle in haystack for needle in AGORA_PROCESS_NEEDLES):
            continue
        try:
            processes.append({"pid": int(pid), "command": command, "rssKb": int(rss), "vszKb": int(vsz), "args": args[:220]})
        except ValueError:
            continue
    return processes


def capture_output(args: argparse.Namespace, checked_at: int, surface_ids: list[str]) -> tuple[dict, dict | None]:
    structured = load_module("check-structured-layout.py", "agora_de_check_structured_layout")
    if args.capture_delay_seconds > 0:
        time.sleep(args.capture_delay_seconds)
    capture_check, packet = structured.capture_visible_structured_layout(
        args.compositorctl,
        args.output_name,
        args.output_capture_session,
        checked_at,
        surface_ids,
    )
    capture_check = dict(capture_check)
    if capture_check.get("status") == "pass":
        capture_check["name"] = "capture"
        capture_check["detail"] = "physical output capture shows live-session soak state"
    if packet:
        packet = dict(packet)
        packet["scenario"] = SCENARIO
        packet["surfaceIds"] = surface_ids
    return capture_check, packet


def collect_journals(args: argparse.Namespace) -> dict:
    units = [
        ("user-shellui", ["journalctl", "--user", "-u", "agora-de-shellui.service", "-n", str(args.journal_lines), "--no-pager"]),
        ("user-panel", ["journalctl", "--user", "-u", "agora-de-shell-panel.service", "-n", str(args.journal_lines), "--no-pager"]),
        ("user-overlay", ["journalctl", "--user", "-u", "agora-de-shell-overlay.service", "-n", str(args.journal_lines), "--no-pager"]),
        ("system-bridge", ["journalctl", "-u", "compositor-bridge.service", "-n", str(args.journal_lines), "--no-pager"]),
    ]
    journals = {}
    for name, command in units:
        try:
            completed = subprocess.run(command, check=False, text=True, capture_output=True, timeout=5)
            journals[name] = {
                "command": command,
                "returnCode": completed.returncode,
                "stdout": completed.stdout,
                "stderr": completed.stderr,
            }
        except (OSError, subprocess.TimeoutExpired) as error:
            journals[name] = {"command": command, "returnCode": -1, "stdout": "", "stderr": str(error)}
    return journals


def write_artifacts(args: argparse.Namespace, result: dict, samples: list[dict], journals: dict) -> list[str]:
    if not args.artifact_dir:
        return []
    artifact_dir = pathlib.Path(args.artifact_dir)
    artifact_dir.mkdir(parents=True, exist_ok=True)
    paths = []
    summary_path = artifact_dir / "summary.json"
    summary_path.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    paths.append(str(summary_path))
    samples_path = artifact_dir / "samples.jsonl"
    with samples_path.open("w", encoding="utf-8") as handle:
        for sample in samples:
            handle.write(json.dumps(sample, sort_keys=True) + "\n")
    paths.append(str(samples_path))
    for name, journal in journals.items():
        journal_path = artifact_dir / f"journal-{name}.log"
        journal_path.write_text(journal.get("stdout", "") + journal.get("stderr", ""), encoding="utf-8")
        paths.append(str(journal_path))
    packets = result.get("evidencePackets") or []
    if packets:
        packet_path = artifact_dir / "capture-packets.json"
        packet_path.write_text(json.dumps(packets, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        paths.append(str(packet_path))
    return paths


def check_compositorctl_path(compositorctl: str) -> dict:
    path = pathlib.Path(compositorctl)
    try:
        resolved = path.resolve(strict=False)
    except OSError:
        resolved = path
    if path == OLD_COMPOSITORCTL or resolved == OLD_COMPOSITORCTL:
        return failed("compositorctl", "old /usr/local/bin/compositorctl path is not allowed")
    if path.name != "agora-de-compositorctl":
        return failed("compositorctl", "live-session soak evidence must use agora-de-compositorctl", path=str(path))
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
    return passed("compositorctl", "compositorctl is available for installed-session soak evidence", path=str(path))


def load_module(filename: str, module_name: str):
    path = pathlib.Path(__file__).with_name(filename)
    spec = importlib.util.spec_from_file_location(module_name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def active_work_surface_ids(base_url: str) -> list[str]:
    try:
        surfaces = get_json(base_url + "/api/surfaces").get("surfaces") or []
    except Exception:
        return []
    return [
        surface.get("id")
        for surface in surfaces
        if surface.get("id")
        and surface.get("mapped") is not False
        and surface.get("appId") not in {"io.agorade.ShellPanel", "io.agorade.ShellOverlay"}
    ]


def close_shell_popups(base_url: str) -> None:
    for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
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


def wait_for_surface(
    base_url: str,
    surface_id: str,
    expected_app_id: str,
    timeout_seconds: float,
    predicate=None,
) -> tuple[dict | None, float]:
    started = time.perf_counter()
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        for surface in get_json(base_url + "/api/surfaces").get("surfaces", []):
            if surface.get("id") != surface_id or surface.get("appId") != expected_app_id:
                continue
            if predicate is not None and not predicate(surface):
                continue
            if predicate is None and surface.get("mapped") is False:
                continue
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


def get_text(url: str) -> str:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-live-session-soak/1"})
    with urllib.request.urlopen(request, timeout=5) as response:
        return response.read().decode("utf-8")


def get_json(url: str) -> dict:
    return json.loads(get_text(url))


def post_json(url: str, body: dict) -> dict:
    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        method="POST",
        headers={"Content-Type": "application/json", "User-Agent": "agora-de-live-session-soak/1"},
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{url} returned {error.code}: {detail}") from error


def named_cycle_check(check: dict, cycle: int) -> dict:
    updated = dict(check)
    updated["name"] = f"cycle-{cycle}:{check.get('name', 'check')}"
    updated["cycle"] = cycle
    return updated


def elapsed_ms(started: float) -> float:
    return (time.perf_counter() - started) * 1000


def unix_millis() -> int:
    return int(time.time() * 1000)


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "soak", "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "soak", "status": "fail", "detail": detail, **extra}


def finish(
    args: argparse.Namespace,
    checked_at: int,
    app_ids: list[str],
    expected_app_ids: list[str],
    checks: list[dict],
    samples: list[dict],
    events: list[dict],
    evidence_packets: list[dict],
) -> int:
    journals = collect_journals(args)
    failed_checks = [check for check in checks if check.get("status") == "fail"]
    result = {
        "schema": SCHEMA,
        "scenario": SCENARIO,
        "checkedAtUnixMillis": checked_at,
        "baseUrl": args.base_url,
        "cycles": args.cycles,
        "appIds": app_ids,
        "expectedAppIds": expected_app_ids,
        "checks": checks,
        "samples": samples,
        "events": events,
        "evidencePackets": evidence_packets,
        "journals": {
            name: {
                "command": journal.get("command", []),
                "returnCode": journal.get("returnCode", -1),
                "stdoutTail": journal.get("stdout", "")[-3000:],
                "stderrTail": journal.get("stderr", "")[-3000:],
            }
            for name, journal in journals.items()
        },
        "summary": {
            "status": "fail" if failed_checks else "pass",
            "passed": len(checks) - len(failed_checks),
            "failed": len(failed_checks),
        },
    }
    artifact_paths = write_artifacts(args, result, samples, journals)
    if artifact_paths:
        result["artifactPaths"] = artifact_paths
        summary_path = pathlib.Path(artifact_paths[0])
        summary_path.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 1 if failed_checks else 0


if __name__ == "__main__":
    raise SystemExit(main())

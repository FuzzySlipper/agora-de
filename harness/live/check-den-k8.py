#!/usr/bin/env python3
import argparse
import binascii
import json
import os
import pathlib
import socket
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.request
import zlib


DEFAULT_UNITS = [
    "event-bus.service",
    "event-bus-web.service",
    "compositor-bridge.service",
    "agora-wayfire.service",
    "agora-shell-panel.service",
]

DEFAULT_SOCKETS = [
    "/run/agent-os/bus.sock",
    "/run/agent-os/compositor-bridge.sock",
    "/run/agent-os/compositor-control.sock",
]


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Run agora-de live evidence checks against the installed den-k8 service."
    )
    parser.add_argument(
        "--shell-url",
        default=os.environ.get(
            "AGORA_DE_LIVE_SHELL_URL",
            "http://127.0.0.1:7780/shell/dist/desktop/?surface=dock",
        ),
        help="Installed shell URL to check.",
    )
    parser.add_argument(
        "--systemd-units",
        default=os.environ.get("AGORA_DE_LIVE_SYSTEMD_UNITS", ",".join(DEFAULT_UNITS)),
        help="Comma-separated systemd service units expected to be active.",
    )
    parser.add_argument(
        "--sockets",
        default=os.environ.get("AGORA_DE_LIVE_SOCKETS", ",".join(DEFAULT_SOCKETS)),
        help="Comma-separated Unix sockets expected from the installed service.",
    )
    parser.add_argument(
        "--capture-json",
        default=os.environ.get("AGORA_DE_LIVE_CAPTURE_JSON", ""),
        help="Optional installed-service capture JSON to classify into an EvidencePacket.",
    )
    parser.add_argument(
        "--output-name",
        default=os.environ.get("AGORA_DE_LIVE_OUTPUT_NAME", ""),
        help="Optional physical compositor output name to capture through compositorctl output capture.",
    )
    parser.add_argument(
        "--output-capture-session",
        default=os.environ.get("AGORA_DE_LIVE_OUTPUT_CAPTURE_SESSION", "den-k8-live-output"),
        help="Artifact session id used with --output-name capture.",
    )
    parser.add_argument(
        "--compositorctl",
        default=os.environ.get("AGORA_DE_LIVE_COMPOSITORCTL", "/home/agent/.local/bin/agora-de-compositorctl"),
        help="compositorctl binary used for optional surface readback checks.",
    )
    parser.add_argument(
        "--surface-app-id",
        default=os.environ.get("AGORA_DE_LIVE_SURFACE_APP_ID", ""),
        help="Optional compositor surface app id expected to be mapped.",
    )
    parser.add_argument(
        "--surface-role",
        default=os.environ.get("AGORA_DE_LIVE_SURFACE_ROLE", ""),
        help="Optional compositor surface role expected with --surface-app-id.",
    )
    parser.add_argument(
        "--require-frame",
        action="store_true",
        default=os.environ.get("AGORA_DE_LIVE_REQUIRE_FRAME", "") == "1",
        help="Fail when the selected compositor surface has not presented a frame.",
    )
    parser.add_argument(
        "--catalog-url",
        default=os.environ.get("AGORA_DE_LIVE_CATALOG_URL", ""),
        help="Optional installed app catalog JSON route to validate.",
    )
    parser.add_argument(
        "--surfaces-url",
        default=os.environ.get("AGORA_DE_LIVE_SURFACES_URL", ""),
        help="Optional installed surface lifecycle JSON route to validate.",
    )
    parser.add_argument(
        "--work-controls-url",
        default=os.environ.get("AGORA_DE_LIVE_WORK_CONTROLS_URL", ""),
        help="Optional installed work surface controls JSON route to validate.",
    )
    parser.add_argument(
        "--workspaces-url",
        default=os.environ.get("AGORA_DE_LIVE_WORKSPACES_URL", ""),
        help="Optional installed workspace JSON route to validate.",
    )
    parser.add_argument(
        "--operator-status-url",
        default=os.environ.get("AGORA_DE_LIVE_OPERATOR_STATUS_URL", ""),
        help="Optional installed operator status JSON route to validate.",
    )
    parser.add_argument(
        "--require-capture",
        action="store_true",
        default=os.environ.get("AGORA_DE_LIVE_REQUIRE_CAPTURE", "") == "1",
        help="Fail when neither --capture-json nor --output-name supplies classifiable capture evidence.",
    )
    parser.add_argument(
        "--timeout-seconds",
        type=float,
        default=float(os.environ.get("AGORA_DE_LIVE_TIMEOUT_SECONDS", "3")),
        help="HTTP/socket timeout.",
    )
    args = parser.parse_args()

    checked_at = unix_millis()
    checks = []
    evidence_packets = []

    for unit in csv(args.systemd_units):
        checks.append(check_systemd_unit(unit))

    for path in csv(args.sockets):
        checks.append(check_socket(path, args.timeout_seconds))

    shell_check = check_http_shell(args.shell_url, args.timeout_seconds)
    checks.append(shell_check)
    evidence_packets.append(
        {
            "scenario": "den-k8-shell-http-installed-service",
            "capturedAtUnixMillis": checked_at,
            "visualStatus": "unknown",
            "captureClassification": "insufficient_mapped_only",
        }
    )

    if args.capture_json:
        capture_check, packet = classify_capture_json(pathlib.Path(args.capture_json), checked_at)
        checks.append(capture_check)
        if packet:
            evidence_packets.append(packet)
    elif args.output_name:
        capture_check, packet = capture_and_classify_output(
            args.compositorctl,
            args.output_name,
            args.output_capture_session,
            checked_at,
        )
        checks.append(capture_check)
        if packet:
            evidence_packets.append(packet)
    elif args.require_capture:
        checks.append(
            failed_check(
                "capture",
                "capture",
                "AGORA_DE_LIVE_CAPTURE_JSON/--capture-json or AGORA_DE_LIVE_OUTPUT_NAME/--output-name is required for capture evidence",
            )
        )

    if args.surface_app_id:
        surface_check, packet = check_compositor_surface(
            args.compositorctl,
            args.surface_app_id,
            args.surface_role,
            args.require_frame,
            checked_at,
        )
        checks.append(surface_check)
        if packet:
            evidence_packets.append(packet)

    route_claims = [
        (
            args.catalog_url,
            "app-catalog-route",
            "den-k8-app-catalog-route",
            validate_catalog_payload,
        ),
        (
            args.surfaces_url,
            "surface-lifecycle-route",
            "den-k8-surface-lifecycle-route",
            validate_surfaces_payload,
        ),
        (
            args.work_controls_url,
            "work-surface-controls-route",
            "den-k8-work-surface-controls-route",
            validate_surfaces_payload,
        ),
        (
            args.workspaces_url,
            "workspaces-route",
            "den-k8-workspaces-route",
            validate_workspaces_payload,
        ),
        (
            args.operator_status_url,
            "operator-status-route",
            "den-k8-operator-status-route",
            validate_operator_status_payload,
        ),
    ]
    for url, name, scenario, validator in route_claims:
        if not url:
            continue
        route_check, packet = check_json_claim_route(
            url,
            name,
            scenario,
            args.timeout_seconds,
            checked_at,
            validator,
        )
        checks.append(route_check)
        if packet:
            evidence_packets.append(packet)

    failed = [check for check in checks if check["status"] != "pass"]
    result = {
        "schema": "agora-de.den-k8-live-evidence.v1",
        "host": socket.gethostname(),
        "checkedAtUnixMillis": checked_at,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "summary": {
            "status": "fail" if failed else "pass",
            "passed": len(checks) - len(failed),
            "failed": len(failed),
        },
    }
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 1 if failed else 0


def csv(value: str) -> list[str]:
    return [item.strip() for item in value.split(",") if item.strip()]


def unix_millis() -> int:
    return int(time.time() * 1000)


def passed_check(name: str, category: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": category, "status": "pass", "detail": detail, **extra}


def failed_check(name: str, category: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": category, "status": "fail", "detail": detail, **extra}


def check_systemd_unit(unit: str) -> dict:
    try:
        completed = subprocess.run(
            ["systemctl", "is-active", unit],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=5,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        return failed_check(unit, "service", f"systemctl failed: {error}")

    state = completed.stdout.strip() or completed.stderr.strip()
    if completed.returncode == 0 and state == "active":
        return passed_check(unit, "service", "systemd unit is active", state=state)
    return failed_check(unit, "service", "systemd unit is not active", state=state)


def check_socket(path_value: str, timeout_seconds: float) -> dict:
    path = pathlib.Path(path_value)
    if not path.exists():
        return failed_check(path_value, "compositor", "socket does not exist")
    if not path.is_socket():
        return failed_check(path_value, "compositor", "path exists but is not a socket")

    client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    client.settimeout(timeout_seconds)
    try:
        client.connect(path_value)
    except OSError as error:
        return failed_check(path_value, "compositor", f"socket exists but connect failed: {error}")
    finally:
        client.close()
    return passed_check(path_value, "compositor", "socket accepts a Unix connection")


def check_http_shell(url: str, timeout_seconds: float) -> dict:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-live-evidence/1"})
    try:
        with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
            body = response.read(512).decode("utf-8", errors="replace")
            status = response.status
            content_type = response.headers.get("Content-Type", "")
    except urllib.error.HTTPError as error:
        return failed_check(url, "shell-ui", f"HTTP status {error.code}", httpStatus=error.code)
    except urllib.error.URLError as error:
        return failed_check(url, "shell-ui", f"HTTP request failed: {error.reason}")

    if status != 200:
        return failed_check(url, "shell-ui", f"HTTP status {status}", httpStatus=status)
    if "<!doctype html>" not in body.lower():
        return failed_check(url, "shell-ui", "response did not look like shell HTML", httpStatus=status)
    return passed_check(url, "shell-ui", "shell HTML route is available", httpStatus=status, contentType=content_type)


def check_json_claim_route(url: str, name: str, scenario: str, timeout_seconds: float, checked_at: int, validator) -> tuple[dict, dict | None]:
    request = urllib.request.Request(url, headers={"User-Agent": "agora-de-live-evidence/1"})
    try:
        with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
            body = response.read(256 * 1024).decode("utf-8", errors="replace")
            status = response.status
            content_type = response.headers.get("Content-Type", "")
    except urllib.error.HTTPError as error:
        return failed_check(url, name, f"HTTP status {error.code}", httpStatus=error.code), unavailable_packet(
            scenario,
            checked_at,
        )
    except urllib.error.URLError as error:
        return failed_check(url, name, f"HTTP request failed: {error.reason}"), unavailable_packet(scenario, checked_at)

    if status != 200:
        return failed_check(url, name, f"HTTP status {status}", httpStatus=status), unavailable_packet(
            scenario,
            checked_at,
        )

    try:
        payload = json.loads(body)
    except json.JSONDecodeError as error:
        return failed_check(url, name, f"invalid JSON response: {error}", httpStatus=status), unavailable_packet(
            scenario,
            checked_at,
        )

    validation_error = validator(payload)
    if validation_error:
        return failed_check(
            url,
            name,
            validation_error,
            httpStatus=status,
            contentType=content_type,
        ), unavailable_packet(scenario, checked_at)

    return passed_check(
        url,
        name,
        "installed JSON claim route returned valid data",
        httpStatus=status,
        contentType=content_type,
    ), {
        "scenario": scenario,
        "capturedAtUnixMillis": checked_at,
        "visualStatus": "unknown",
        "captureClassification": "insufficient_mapped_only",
    }


def validate_catalog_payload(payload: object) -> str | None:
    if not isinstance(payload, dict) or not isinstance(payload.get("apps"), list):
        return "catalog route response must contain apps array"
    for app in payload["apps"]:
        if not isinstance(app, dict):
            return "catalog route app entry must be an object"
        for field in ("id", "name", "icon"):
            if not isinstance(app.get(field), str):
                return f"catalog route app entry missing string {field}"
    return None


def validate_surfaces_payload(payload: object) -> str | None:
    if not isinstance(payload, dict) or not isinstance(payload.get("surfaces"), list):
        return "surface route response must contain surfaces array"
    for surface in payload["surfaces"]:
        if not isinstance(surface, dict):
            return "surface route entry must be an object"
        if not isinstance(surface.get("id"), str):
            return "surface route entry missing string id"
        if not isinstance(surface.get("ownerUid"), int):
            return "surface route entry missing integer ownerUid"
        for field in ("mapped", "focused"):
            if not isinstance(surface.get(field), bool):
                return f"surface route entry missing boolean {field}"
        if not isinstance(surface.get("inputDeniedCount"), int):
            return "surface route entry missing integer inputDeniedCount"
    return None


def validate_operator_status_payload(payload: object) -> str | None:
    if not isinstance(payload, dict):
        return "operator status route response must be an object"
    if not isinstance(payload.get("overall"), str) or not payload["overall"]:
        return "operator status route missing overall state"
    if not isinstance(payload.get("services"), list) or not payload["services"]:
        return "operator status route response must contain services array"
    if not isinstance(payload.get("sockets"), list) or not payload["sockets"]:
        return "operator status route response must contain sockets array"
    surfaces = payload.get("surfaces")
    if not isinstance(surfaces, dict) or not isinstance(surfaces.get("state"), str):
        return "operator status route response must contain surface summary"
    recovery = payload.get("recovery")
    if not isinstance(recovery, dict):
        return "operator status route response must contain recovery object"
    if recovery.get("killAllCommand") != "sudo /usr/local/sbin/agora-de-kill-all":
        return "operator status route must reference durable kill-all helper"
    if not isinstance(recovery.get("restartCommands"), list) or not recovery["restartCommands"]:
        return "operator status route must include restart commands"
    if not isinstance(recovery.get("runbook"), str) or "den-k8-visible-shell-runbook.md" not in recovery["runbook"]:
        return "operator status route must reference visible-shell runbook"
    for service in payload["services"]:
        if not isinstance(service, dict):
            return "operator status service entry must be an object"
        for field in ("name", "scope", "state"):
            if not isinstance(service.get(field), str):
                return f"operator status service entry missing string {field}"
    for socket_status in payload["sockets"]:
        if not isinstance(socket_status, dict):
            return "operator status socket entry must be an object"
        for field in ("path", "state"):
            if not isinstance(socket_status.get(field), str):
                return f"operator status socket entry missing string {field}"
    return None


def validate_workspaces_payload(payload: object) -> str | None:
    if not isinstance(payload, dict):
        return "workspace route response must be an object"
    if payload.get("currentWorkspaceId") != "workspace-1":
        return "workspace route currentWorkspaceId must be workspace-1"
    workspaces = payload.get("workspaces")
    if not isinstance(workspaces, list) or not workspaces:
        return "workspace route response must contain workspaces array"
    active = [workspace for workspace in workspaces if isinstance(workspace, dict) and workspace.get("active")]
    if len(active) != 1:
        return "workspace route must report exactly one active workspace"
    workspace = active[0]
    if workspace.get("id") != "workspace-1":
        return "workspace route active workspace must be workspace-1"
    if not isinstance(workspace.get("name"), str) or not workspace["name"]:
        return "workspace route workspace missing name"
    if not isinstance(workspace.get("surfaceCount"), int):
        return "workspace route workspace missing integer surfaceCount"
    return None


def unavailable_packet(scenario: str, checked_at: int) -> dict:
    return {
        "scenario": scenario,
        "capturedAtUnixMillis": checked_at,
        "visualStatus": "unknown",
        "captureClassification": "not_visible",
    }


def check_compositor_surface(
    compositorctl: str,
    app_id: str,
    role: str,
    require_frame: bool,
    checked_at: int,
) -> tuple[dict, dict | None]:
    try:
        completed = subprocess.run(
            [compositorctl, "list-surfaces"],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=5,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        return failed_check(app_id, "surface-readback", f"compositorctl failed: {error}"), unavailable_packet(
            "den-k8-compositor-surface-readback",
            checked_at,
        )

    if completed.returncode != 0:
        return failed_check(
            app_id,
            "surface-readback",
            "compositorctl list-surfaces failed",
            stderr=completed.stderr.strip(),
        ), unavailable_packet("den-k8-compositor-surface-readback", checked_at)

    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        return failed_check(app_id, "surface-readback", f"invalid compositorctl JSON: {error}"), unavailable_packet(
            "den-k8-compositor-surface-readback",
            checked_at,
        )

    matches = []
    for item in payload.get("surfaces", []):
        surface = item.get("surface") or {}
        if surface.get("app_id") != app_id:
            continue
        if role and surface.get("role") != role:
            continue
        matches.append(item)

    if not matches:
        return failed_check(app_id, "surface-readback", "matching compositor surface was not mapped"), unavailable_packet(
            "den-k8-compositor-surface-readback",
            checked_at,
        )

    selected = sorted(matches, key=lambda item: item.get("updated_at") or "")[-1]
    surface = selected.get("surface") or {}
    frame_count = int(selected.get("frame_count") or 0)
    content_commit_count = int(selected.get("content_commit_count") or 0)
    visible = bool(selected.get("visible") or surface.get("visible"))
    surface_id = surface.get("id") or app_id
    classification = "insufficient_mapped_only"
    if content_commit_count > 0:
        classification = "content_committed"
    if frame_count > 0:
        classification = "frame_presented"

    packet = {
        "scenario": "den-k8-compositor-surface-readback",
        "capturedAtUnixMillis": checked_at,
        "surfaceId": surface_id,
        "visualStatus": "unknown",
        "frameCount": frame_count,
        "contentCommitCount": content_commit_count,
        "captureClassification": classification,
    }

    if not visible:
        packet["captureClassification"] = "not_visible"
        return failed_check(
            surface_id,
            "surface-readback",
            "matching compositor surface is mapped but not visible",
            appId=app_id,
            role=surface.get("role"),
            frameCount=frame_count,
            contentCommitCount=content_commit_count,
        ), packet

    if require_frame and frame_count <= 0:
        return failed_check(
            surface_id,
            "surface-readback",
            "matching compositor surface is visible but has not presented a frame",
            appId=app_id,
            role=surface.get("role"),
            frameCount=frame_count,
            contentCommitCount=content_commit_count,
        ), packet

    if frame_count > 0:
        return passed_check(
            surface_id,
            "surface-readback",
            "matching compositor surface is visible and has presented a frame",
            appId=app_id,
            role=surface.get("role"),
            frameCount=frame_count,
            contentCommitCount=content_commit_count,
        ), packet

    if content_commit_count > 0:
        return passed_check(
            surface_id,
            "surface-readback",
            "matching compositor surface is visible and has content commit evidence",
            appId=app_id,
            role=surface.get("role"),
            frameCount=frame_count,
            contentCommitCount=content_commit_count,
        ), packet

    return passed_check(
        surface_id,
        "surface-readback",
        "matching compositor surface is visible but has no frame evidence",
        appId=app_id,
        role=surface.get("role"),
        frameCount=frame_count,
        contentCommitCount=content_commit_count,
    ), packet


def capture_and_classify_output(
    compositorctl: str,
    output_name: str,
    session_id: str,
    checked_at: int,
) -> tuple[dict, dict | None]:
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
        return failed_check(output_name, "capture", f"compositorctl output capture failed: {error}"), unavailable_packet(
            "den-k8-installed-service-capture",
            checked_at,
        )

    if completed.returncode != 0:
        return failed_check(
            output_name,
            "capture",
            "compositorctl output capture failed",
            stderr=completed.stderr.strip(),
        ), unavailable_packet("den-k8-installed-service-capture", checked_at)

    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        return failed_check(output_name, "capture", f"invalid output capture JSON: {error}"), unavailable_packet(
            "den-k8-installed-service-capture",
            checked_at,
        )

    capture = first_capture_record(payload)
    if not capture:
        return failed_check(output_name, "capture", "output capture did not return any capture artifacts"), unavailable_packet(
            "den-k8-installed-service-capture",
            checked_at,
        )
    return classify_capture_record(capture, checked_at, output_name=output_name, require_shell_pixels=True)


def classify_capture_json(path: pathlib.Path, checked_at: int) -> tuple[dict, dict | None]:
    try:
        capture = json.loads(path.read_text())
    except (OSError, json.JSONDecodeError) as error:
        return failed_check(str(path), "capture", f"capture JSON could not be read: {error}"), None
    if isinstance(capture, dict) and isinstance(capture.get("evidencePackets"), list):
        return classify_evidence_summary_capture(path, capture, checked_at)
    capture = first_capture_record(capture) or capture
    return classify_capture_record(capture, checked_at, source_name=str(path), require_shell_pixels=False)


def classify_evidence_summary_capture(path: pathlib.Path, summary: dict, checked_at: int) -> tuple[dict, dict | None]:
    packets = [packet for packet in summary.get("evidencePackets", []) if isinstance(packet, dict)]
    capture_packets = [packet for packet in packets if packet.get("scenario") == "den-k8-installed-service-capture"]
    if not capture_packets:
        return failed_check(str(path), "capture", "evidence summary contains no installed-service capture packet"), None
    packet = dict(capture_packets[-1])
    packet["capturedAtUnixMillis"] = checked_at
    visual_status = packet.get("visualStatus", "unknown")
    classification = packet.get("captureClassification", "not_visible")
    if visual_status == "visible" and classification == "capture_visible":
        return passed_check(str(path), "capture", "evidence summary reports visible capture"), packet
    if classification == "blank_capture_failure":
        return failed_check(str(path), "capture", "evidence summary reports blank capture"), packet
    return failed_check(str(path), "capture", "evidence summary does not prove visibility"), packet


def first_capture_record(payload: object) -> dict | None:
    if not isinstance(payload, dict):
        return None
    captures = payload.get("captures")
    if isinstance(captures, list) and captures:
        capture = captures[0]
        if isinstance(capture, dict):
            artifact = capture.get("artifact")
            if isinstance(artifact, dict):
                merged = dict(artifact)
                merged.update({key: value for key, value in capture.items() if key != "artifact"})
                return merged
            return capture
    return payload


def classify_capture_record(
    capture: dict,
    checked_at: int,
    source_name: str | None = None,
    output_name: str | None = None,
    require_shell_pixels: bool = False,
) -> tuple[dict, dict | None]:
    name = source_name or output_name or capture.get("image_path") or capture.get("path") or "capture"
    visual_status = capture.get("visual_inspection", {}).get("status", "unknown")
    if visual_status not in {"visible", "blank", "unknown"}:
        visual_status = "unknown"

    image_path = capture.get("image_path") or capture.get("path")
    shell_pixels = None
    pixel_error = ""
    if image_path:
        shell_pixels, pixel_error = classify_shell_output_pixels(pathlib.Path(str(image_path)))

    if visual_status == "visible":
        if require_shell_pixels and (not shell_pixels or not shell_pixels.get("shellVisible")):
            classification = "not_visible"
            detail = "output capture pixels do not match expected shell"
            if pixel_error:
                detail = f"{detail}: {pixel_error}"
            check = failed_check(str(name), "capture", detail, imagePath=image_path, outputName=output_name)
        else:
            classification = "capture_visible"
            if shell_pixels and shell_pixels.get("shellVisible"):
                check = passed_check(
                    str(name),
                    "capture",
                    "output capture shows expected shell pixels",
                    imagePath=image_path,
                    outputName=output_name,
                )
            else:
                check = passed_check(str(name), "capture", "capture JSON reports visible inspection", imagePath=image_path)
    elif visual_status == "blank":
        classification = "blank_capture_failure"
        check = failed_check(str(name), "capture", "capture JSON reports blank inspection", imagePath=image_path)
    else:
        classification = "not_visible"
        check = failed_check(str(name), "capture", "capture JSON does not prove visibility", imagePath=image_path)

    packet = {
        "scenario": "den-k8-installed-service-capture",
        "capturedAtUnixMillis": checked_at,
        "visualStatus": visual_status,
        "captureClassification": classification,
    }
    if output_name:
        packet["outputName"] = output_name
    if image_path:
        packet["artifactPath"] = image_path
    if shell_pixels:
        packet["pixelClassification"] = shell_pixels
    elif pixel_error:
        packet["pixelClassificationError"] = pixel_error
    return check, packet


def classify_shell_output_pixels(path: pathlib.Path) -> tuple[dict | None, str]:
    try:
        image = read_png_rgb(path)
    except (OSError, ValueError, zlib.error, binascii.Error, struct.error) as error:
        return None, str(error)

    width = image["width"]
    height = image["height"]
    rows = image["rows"]
    total = width * height
    if total <= 0:
        return None, "capture has empty dimensions"

    light_background = 0
    black_pixels = 0
    evidence_accent_pixels = 0
    evidence_strong_pixels = 0
    text_like_pixels = 0
    main_text_max_y = max(1, int(height * 0.85))
    panel_start = max(0, height - 140)
    min_evidence_accent_pixels = max(128, width // 4)
    min_evidence_strong_pixels = 512
    min_text_like_pixels = 64

    for y, row in enumerate(rows):
        for x in range(width):
            i = x * 3
            r, g, b = row[i], row[i + 1], row[i + 2]
            if r < 10 and g < 10 and b < 10:
                black_pixels += 1
            if r >= 235 and g >= 235 and b >= 235:
                light_background += 1
            if r <= 30 and g >= 170 and b >= 140:
                evidence_accent_pixels += 1
            if y >= panel_start and r <= 45 and g <= 70 and b <= 80:
                evidence_strong_pixels += 1
            dark_text_like = r <= 90 and g <= 110 and b <= 125
            light_text_like = r >= 180 and g >= 190 and b >= 200
            if y < main_text_max_y and x < max(480, width // 4) and (dark_text_like or light_text_like):
                text_like_pixels += 1

    light_ratio = light_background / total
    black_ratio = black_pixels / total
    shell_visible = (
        black_ratio < 0.95
        and evidence_accent_pixels >= min_evidence_accent_pixels
        and evidence_strong_pixels >= min_evidence_strong_pixels
        and text_like_pixels >= min_text_like_pixels
    )

    if shell_visible:
        classification = "expected_shell_visible"
    elif black_ratio >= 0.95:
        classification = "blank_or_black_output"
    else:
        classification = "unexpected_output_pixels"

    return {
        "classification": classification,
        "shellVisible": shell_visible,
        "width": width,
        "height": height,
        "themeEvidenceContract": "agora-de.theme-evidence.v1",
        "lightBackgroundRatio": round(light_ratio, 4),
        "blackPixelRatio": round(black_ratio, 4),
        "evidenceAccentPixels": evidence_accent_pixels,
        "evidenceStrongPixels": evidence_strong_pixels,
        "textLikePixels": text_like_pixels,
    }, ""


def read_png_rgb(path: pathlib.Path) -> dict:
    data = path.read_bytes()
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        raise ValueError("capture artifact is not a PNG")

    pos = 8
    width = height = color_type = bit_depth = interlace = None
    idat = bytearray()
    while pos < len(data):
        if pos + 8 > len(data):
            raise ValueError("truncated PNG chunk")
        length = struct.unpack(">I", data[pos : pos + 4])[0]
        chunk_type = data[pos + 4 : pos + 8]
        pos += 8
        chunk = data[pos : pos + length]
        pos += length
        if pos + 4 > len(data):
            raise ValueError("truncated PNG CRC")
        pos += 4
        if chunk_type == b"IHDR":
            width, height, bit_depth, color_type, _, _, interlace = struct.unpack(">IIBBBBB", chunk)
        elif chunk_type == b"IDAT":
            idat.extend(chunk)
        elif chunk_type == b"IEND":
            break

    if width is None or height is None:
        raise ValueError("PNG missing IHDR")
    if bit_depth != 8 or color_type not in (2, 6) or interlace != 0:
        raise ValueError(f"unsupported PNG format bit_depth={bit_depth} color_type={color_type} interlace={interlace}")

    channels = 4 if color_type == 6 else 3
    stride = width * channels
    raw = zlib.decompress(bytes(idat))
    expected = (stride + 1) * height
    if len(raw) < expected:
        raise ValueError("PNG image data is truncated")

    rows = []
    prev = bytearray(stride)
    offset = 0
    for _ in range(height):
        filter_type = raw[offset]
        offset += 1
        current = bytearray(raw[offset : offset + stride])
        offset += stride
        unfilter_png_row(current, prev, channels, filter_type)
        if channels == 4:
            rgb = bytearray(width * 3)
            for x in range(width):
                src = x * 4
                dst = x * 3
                rgb[dst : dst + 3] = current[src : src + 3]
            rows.append(bytes(rgb))
        else:
            rows.append(bytes(current))
        prev = current

    return {"width": width, "height": height, "rows": rows}


def unfilter_png_row(row: bytearray, prev: bytearray, bpp: int, filter_type: int) -> None:
    for i in range(len(row)):
        left = row[i - bpp] if i >= bpp else 0
        up = prev[i]
        up_left = prev[i - bpp] if i >= bpp else 0
        if filter_type == 0:
            value = row[i]
        elif filter_type == 1:
            value = row[i] + left
        elif filter_type == 2:
            value = row[i] + up
        elif filter_type == 3:
            value = row[i] + ((left + up) // 2)
        elif filter_type == 4:
            value = row[i] + paeth(left, up, up_left)
        else:
            raise ValueError(f"unsupported PNG filter {filter_type}")
        row[i] = value & 0xFF


def paeth(left: int, up: int, up_left: int) -> int:
    estimate = left + up - up_left
    left_distance = abs(estimate - left)
    up_distance = abs(estimate - up)
    up_left_distance = abs(estimate - up_left)
    if left_distance <= up_distance and left_distance <= up_left_distance:
        return left
    if up_distance <= up_left_distance:
        return up
    return up_left


if __name__ == "__main__":
    raise SystemExit(main())

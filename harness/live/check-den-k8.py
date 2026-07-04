#!/usr/bin/env python3
import argparse
import json
import os
import pathlib
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request


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
        "--require-capture",
        action="store_true",
        default=os.environ.get("AGORA_DE_LIVE_REQUIRE_CAPTURE", "") == "1",
        help="Fail when --capture-json is not supplied or cannot be classified.",
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
    elif args.require_capture:
        checks.append(
            failed_check(
                "capture-json",
                "capture",
                "AGORA_DE_LIVE_CAPTURE_JSON or --capture-json is required for capture evidence",
            )
        )

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


def classify_capture_json(path: pathlib.Path, checked_at: int) -> tuple[dict, dict | None]:
    try:
        capture = json.loads(path.read_text())
    except (OSError, json.JSONDecodeError) as error:
        return failed_check(str(path), "capture", f"capture JSON could not be read: {error}"), None

    visual_status = capture.get("visual_inspection", {}).get("status", "unknown")
    if visual_status not in {"visible", "blank", "unknown"}:
        visual_status = "unknown"

    if visual_status == "visible":
        classification = "capture_visible"
        check = passed_check(str(path), "capture", "capture JSON reports visible inspection")
    elif visual_status == "blank":
        classification = "blank_capture_failure"
        check = failed_check(str(path), "capture", "capture JSON reports blank inspection")
    else:
        classification = "not_visible"
        check = failed_check(str(path), "capture", "capture JSON does not prove visibility")

    packet = {
        "scenario": "den-k8-installed-service-capture",
        "capturedAtUnixMillis": checked_at,
        "visualStatus": visual_status,
        "captureClassification": classification,
    }
    return check, packet


if __name__ == "__main__":
    raise SystemExit(main())

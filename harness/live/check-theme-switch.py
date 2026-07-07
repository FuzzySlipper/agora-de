#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import subprocess
import sys
import time
import urllib.request


SCHEMA = "agora-de.theme-switch-live.v1"
THEME_EVIDENCE_CONTRACT = "agora-de.theme-evidence.v1"


def main() -> int:
    parser = argparse.ArgumentParser(description="Check installed-service shell theme switching.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--env-file", default=str(pathlib.Path.home() / ".config/agora-de/shellui.env"))
    parser.add_argument("--default-theme-id", default="agora-default")
    parser.add_argument("--variant-theme-id", default="agora-ember")
    parser.add_argument("--variant-accent", default="#fb923c")
    parser.add_argument("--variant-background", default="#12100f")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--output-name", default="")
    parser.add_argument("--output-capture-session", default="den-k8-theme-switch")
    parser.add_argument("--require-capture", action="store_true")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.5)
    parser.add_argument("--restart-timeout-seconds", type=float, default=10.0)
    args = parser.parse_args()

    checked_at = unix_millis()
    checks: list[dict] = []
    evidence_packets: list[dict] = []
    original_env = read_optional_bytes(pathlib.Path(args.env_file))
    overlay_was_active = systemd_user_active("agora-de-shell-overlay.service")

    try:
        checks.extend(apply_theme_and_verify(args, args.default_theme_id, "default", "", ""))
        checks.extend(apply_theme_and_verify(args, args.variant_theme_id, "variant", args.variant_accent, args.variant_background))
        if args.output_name:
            if overlay_was_active:
                stop_overlay(args.restart_timeout_seconds)
                checks.append(passed("overlay-paused", "paused overlay service for shell theme capture"))
            if args.capture_delay_seconds > 0:
                time.sleep(args.capture_delay_seconds)
            capture_check, packet = capture_variant(args, checked_at)
            checks.append(capture_check)
            if packet:
                evidence_packets.append(packet)
        elif args.require_capture:
            checks.append(failed("capture", "physical output capture is required; pass --output-name"))
    except Exception as error:
        checks.append(failed("theme-switch", f"theme switch probe failed: {error}"))
    finally:
        try:
            restore_env(pathlib.Path(args.env_file), original_env)
            restart_shell_services(args.restart_timeout_seconds)
            if overlay_was_active:
                start_overlay(args.restart_timeout_seconds)
        except Exception as error:
            checks.append(failed("restore", f"failed to restore original shellui env: {error}"))

    failed_checks = [check for check in checks if check.get("status") != "pass"]
    result = {
        "schema": SCHEMA,
        "themeEvidenceContract": THEME_EVIDENCE_CONTRACT,
        "checkedAtUnixMillis": checked_at,
        "checks": checks,
        "evidencePackets": evidence_packets,
        "summary": {
            "status": "fail" if failed_checks else "pass",
            "passed": len(checks) - len(failed_checks),
            "failed": len(failed_checks),
        },
    }
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 1 if failed_checks else 0


def apply_theme_and_verify(args: argparse.Namespace, theme_id: str, label: str, accent: str, background: str) -> list[dict]:
    checks = []
    env_path = pathlib.Path(args.env_file)
    set_env_value(env_path, "AGORA_DE_SHELLUI_THEME_ID", theme_id)
    set_env_value(env_path, "AGORA_DE_SHELLUI_THEME_MANIFEST", "")
    restart_shell_services(args.restart_timeout_seconds)

    theme = wait_for_theme(args.base_url, theme_id, args.restart_timeout_seconds)
    if theme.get("id") == theme_id and theme.get("fallback") is False:
        checks.append(passed(f"{label}-theme-route", f"/api/theme reports {theme_id}", theme=theme))
    else:
        checks.append(failed(f"{label}-theme-route", f"/api/theme did not report {theme_id}", theme=theme))
        return checks

    shell_html = get_text(args.base_url + "/shell/dist/desktop/?surface=dock")
    if theme_id in shell_html or f"--agora-bg:" in shell_html:
        checks.append(passed(f"{label}-shell-css", "shell HTML includes theme token CSS", themeId=theme_id))
    else:
        checks.append(failed(f"{label}-shell-css", "shell HTML did not include theme token CSS", themeId=theme_id))
    if accent and accent not in shell_html:
        checks.append(failed("variant-accent-css", "variant accent token missing from shell HTML", expected=accent))
    elif accent:
        checks.append(passed("variant-accent-css", "variant accent token is present", expected=accent))
    if background and background not in shell_html:
        checks.append(failed("variant-background-css", "variant background token missing from shell HTML", expected=background))
    elif background:
        checks.append(passed("variant-background-css", "variant background token is present", expected=background))
    return checks


def capture_variant(args: argparse.Namespace, checked_at: int) -> tuple[dict, dict | None]:
    live_evidence = load_live_evidence_module()
    capture_check, packet = live_evidence.capture_and_classify_output(
        args.compositorctl,
        args.output_name,
        args.output_capture_session,
        checked_at,
    )
    capture_check = dict(capture_check)
    if capture_check.get("status") == "pass":
        capture_check["name"] = "theme-variant-visible-capture"
        capture_check["detail"] = "physical output capture shows themed shell pixels"
    if packet:
        packet = dict(packet)
        packet["scenario"] = "den-k8-theme-switch-visible"
        packet["themeId"] = args.variant_theme_id
    return capture_check, packet


def wait_for_theme(base_url: str, theme_id: str, timeout_seconds: float) -> dict:
    deadline = time.time() + timeout_seconds
    last = {}
    while time.time() < deadline:
        try:
            last = get_json(base_url + "/api/theme")
            if last.get("id") == theme_id:
                return last
        except Exception:
            pass
        time.sleep(0.25)
    return last


def restart_shell_services(timeout_seconds: float) -> None:
    commands = [
        ["systemctl", "--user", "restart", "agora-de-shellui.service"],
        ["systemctl", "--user", "restart", "agora-de-shell-background.service", "agora-de-shell-panel.service"],
    ]
    for command in commands:
        completed = subprocess.run(command, check=False, text=True, capture_output=True, timeout=timeout_seconds)
        if completed.returncode != 0:
            raise RuntimeError(f"{' '.join(command)} failed: {completed.stderr.strip() or completed.stdout.strip()}")


def systemd_user_active(unit: str) -> bool:
    completed = subprocess.run(["systemctl", "--user", "is-active", "--quiet", unit], check=False)
    return completed.returncode == 0


def stop_overlay(timeout_seconds: float) -> None:
    completed = subprocess.run(
        ["systemctl", "--user", "stop", "agora-de-shell-overlay.service"],
        check=False,
        text=True,
        capture_output=True,
        timeout=timeout_seconds,
    )
    if completed.returncode != 0:
        raise RuntimeError(f"stop overlay failed: {completed.stderr.strip() or completed.stdout.strip()}")


def start_overlay(timeout_seconds: float) -> None:
    completed = subprocess.run(
        ["systemctl", "--user", "start", "agora-de-shell-overlay.service"],
        check=False,
        text=True,
        capture_output=True,
        timeout=timeout_seconds,
    )
    if completed.returncode != 0:
        raise RuntimeError(f"start overlay failed: {completed.stderr.strip() or completed.stdout.strip()}")


def set_env_value(path: pathlib.Path, key: str, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    lines = []
    found = False
    if path.exists():
        lines = path.read_text(encoding="utf-8").splitlines()
    output = []
    for line in lines:
        if line.startswith(key + "="):
            output.append(f"{key}={value}")
            found = True
        else:
            output.append(line)
    if not found:
        output.append(f"{key}={value}")
    path.write_text("\n".join(output) + "\n", encoding="utf-8")


def read_optional_bytes(path: pathlib.Path) -> bytes | None:
    if not path.exists():
        return None
    return path.read_bytes()


def restore_env(path: pathlib.Path, original: bytes | None) -> None:
    if original is None:
        try:
            path.unlink()
        except FileNotFoundError:
            pass
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(original)


def get_json(url: str) -> dict:
    with urllib.request.urlopen(url, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))


def get_text(url: str) -> str:
    with urllib.request.urlopen(url, timeout=5) as response:
        return response.read().decode("utf-8")


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
    return {"name": name, "category": "theme-switch", "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "category": "theme-switch", "status": "fail", "detail": detail, **extra}


if __name__ == "__main__":
    raise SystemExit(main())

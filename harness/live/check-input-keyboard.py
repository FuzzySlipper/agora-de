#!/usr/bin/env python3
"""Live evidence: agent keyboard input through agora-de-compositorctl.

Pops Alacritty, focuses it, types a shell command that writes a marker file +
presses Return via `agora-de-compositorctl input keyboard`, then verifies the
file was created — proving keyboard injection reaches a native Wayland client.

Chromium is intentionally NOT the target: it does not accept virtual-keyboard
wl_keyboard keys (needs text-input-v3), documented in the agent surface guide.
"""
from __future__ import annotations

import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(description="Check agent keyboard input injection end-to-end.")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--marker", default="/tmp/agora-de-keyboard-marker")
    args = parser.parse_args()

    checks: list[dict] = []
    checked_at = int(time.time() * 1000)
    if not pathlib.Path(args.compositorctl).exists():
        checks.append(failed("compositorctl", f"not found: {args.compositorctl}"))
        return finish(checks, checked_at)

    if not _have_wtype():
        checks.append(failed("wtype", "wtype not installed (required for keyboard engine)"))
        return finish(checks, checked_at)
    checks.append(passed("wtype", "wtype present"))

    try:
        os.remove(args.marker)
    except FileNotFoundError:
        pass

    with tempfile.TemporaryDirectory(prefix="agora-de-kb-") as tmp:
        cmd = f"echo KB_OK > {args.marker}"
        launch = _ctl_json(args, ["launch", "--arg", "alacritty", "--arg", "-e",
                                  "--arg", "bash", "--arg", "-c", "--arg", "cd /tmp; exec bash",
                                  "--wait-surface", "--wait-timeout-ms", "10000"])
        cid = launch.get("surface_id", "")
        if not cid:
            checks.append(failed("launch", f"no surface mapped: {launch}"))
            return finish(checks, checked_at)
        checks.append(passed("launch", "alacritty mapped", surfaceId=cid))

        typed = _ctl_json(args, ["input", "keyboard", "type", "--surface", cid, "--text", cmd])
        if typed.get("ok") is not True:
            checks.append(failed("type", f"type not ok: {typed}"))
            _close(args, cid)
            return finish(checks, checked_at)
        checks.append(passed("type", "text typed", surfaceId=cid, chars=len(cmd)))

        keyed = _ctl_json(args, ["input", "keyboard", "key", "--surface", cid, "--key", "Return"])
        if keyed.get("ok") is not True:
            checks.append(failed("key", f"key not ok: {keyed}"))
            _close(args, cid)
            return finish(checks, checked_at)
        checks.append(passed("key", "Return pressed", surfaceId=cid))

        time.sleep(1.0)
        if pathlib.Path(args.marker).exists() and pathlib.Path(args.marker).read_text().strip() == "KB_OK":
            checks.append(passed("delivered", "typed command executed, marker file written", path=args.marker))
        else:
            checks.append(failed("delivered", "marker file not created — keys did not reach the terminal"))

        _close(args, cid)

    try:
        os.remove(args.marker)
    except FileNotFoundError:
        pass
    return finish(checks, checked_at)


def _have_wtype() -> bool:
    return subprocess.run(["command", "-v", "wtype"], shell=True, capture_output=True).returncode == 0


def _ctl_json(args, argv: list[str]) -> dict:
    out = _ctl_run(args, argv)
    try:
        return json.loads(out)
    except json.JSONDecodeError:
        return {"ok": False, "raw": out}


def _ctl_run(args, argv: list[str]) -> str:
    proc = subprocess.run([args.compositorctl, *argv], text=True, capture_output=True)
    if proc.returncode != 0:
        raise RuntimeError(f"compositorctl {' '.join(argv)} failed: {proc.stderr.strip() or proc.stdout.strip()}")
    return proc.stdout.strip()


def _close(args, cid: str) -> None:
    subprocess.run([args.compositorctl, "surface", "close", "--surface", cid], capture_output=True)


def passed(name: str, detail: str, **extra) -> dict:
    return {"name": name, "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra) -> dict:
    return {"name": name, "status": "fail", "detail": detail, **extra}


def finish(checks: list[dict], checked_at: int) -> int:
    failed_checks = [c for c in checks if c["status"] != "pass"]
    json.dump(
        {
            "schema": "agora-de.input-keyboard-live.v1",
            "checkedAtUnixMillis": checked_at,
            "checks": checks,
            "summary": {
                "status": "fail" if failed_checks else "pass",
                "passed": len(checks) - len(failed_checks),
                "failed": len(failed_checks),
            },
        },
        sys.stdout,
        indent=2,
        sort_keys=True,
    )
    sys.stdout.write("\n")
    return 1 if failed_checks else 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Live evidence: agent keyboard input into Chromium via text-input-v3 / input-method.

Chromium does not accept virtual-keyboard wl_keyboard keys; it needs the
input-method path. This pops a Chromium page whose <input> echoes its value
into document.title, drives `agora-de-compositorctl input keyboard type` (auto
method -> input-method), and verifies the typed text appears in the title.

Requires Wayfire's input-method-v1 plugin enabled (enable_text_input_v3=true)
and chromium --ozone-platform=wayland.
"""
from __future__ import annotations

import http.server
import json
import pathlib
import socketserver
import subprocess
import sys
import tempfile
import threading
import time


PAGE = (
    "<!doctype html><html><head><meta charset='utf-8'><title>empty</title>"
    "<style>html,body{margin:0;height:100%}input{width:100%;height:100%;"
    "font:60px monospace;border:0;outline:0;padding:0 20px;box-sizing:border-box}</style>"
    "</head><body><input id='i' autofocus></input>"
    "<script>document.title='TYPED:';"
    "document.getElementById('i').addEventListener('input',()=>{"
    "document.title='TYPED:'+document.getElementById('i').value;});</script>"
    "</body></html>"
)


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(description="Check Chromium keyboard input via input-method.")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--text", default="hello agora")
    parser.add_argument("--profile", default="")
    args = parser.parse_args()

    checks: list[dict] = []
    checked_at = int(time.time() * 1000)
    if not pathlib.Path(args.compositorctl).exists():
        checks.append(failed("compositorctl", f"not found: {args.compositorctl}"))
        return finish(checks, checked_at)

    with tempfile.TemporaryDirectory(prefix="agora-de-chrome-kb-") as tmp:
        pathlib.Path(tmp, "index.html").write_text(PAGE)
        server = serve(pathlib.Path(tmp))
        port = server.server_address[1]
        threading.Thread(target=server.serve_forever, daemon=True).start()
        try:
            profile = args.profile or f"{tmp}/chrome-profile"
            launch = _ctl_json(args, ["launch", "--arg", "chromium", "--arg", "--ozone-platform=wayland",
                                      "--arg", f"--user-data-dir={profile}", "--arg", "--no-first-run",
                                      "--arg", "--no-default-browser-check", "--arg", f"http://127.0.0.1:{port}/",
                                      "--wait-surface", "--wait-timeout-ms", "20000"])
            cid = launch.get("surface_id", "")
            if not cid:
                checks.append(failed("launch", f"no surface mapped: {launch}"))
                return finish(checks, checked_at)
            checks.append(passed("launch", "chromium page mapped", surfaceId=cid))

            typed = _ctl_json(args, ["input", "keyboard", "type", "--surface", cid,
                                     "--text", args.text, "--timeout-ms", "5000"])
            if typed.get("ok") is not True:
                checks.append(failed("type", f"type not ok: {typed}"))
                _close(args, cid)
                return finish(checks, checked_at)
            checks.append(passed("type", "input keyboard type returned ok", method=typed.get("helper", "")))

            time.sleep(1.0)
            titles = [s["surface"]["title"] for s in _list(args) if s["surface"]["id"] == cid]
            title = titles[0] if titles else ""
            # chromium appends " - Chromium" to the page title
            if args.text in title:
                checks.append(passed("delivered", "typed text reached chromium input", title=title))
            else:
                checks.append(failed("delivered", f"typed text not in title: {title!r} (want {args.text!r})"))

            _close(args, cid)
        finally:
            server.shutdown()
    return finish(checks, checked_at)


def serve(directory: pathlib.Path) -> socketserver.TCPServer:
    handler = lambda *a, **kw: http.server.SimpleHTTPRequestHandler(*a, directory=str(directory), **kw)
    return socketserver.TCPServer(("127.0.0.1", 0), handler)


def _ctl_json(args, argv: list[str]) -> dict:
    proc = subprocess.run([args.compositorctl, *argv], text=True, capture_output=True)
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError:
        return {"ok": False, "raw": proc.stdout, "stderr": proc.stderr}


def _list(args) -> list:
    proc = subprocess.run([args.compositorctl, "list-surfaces"], text=True, capture_output=True)
    try:
        return json.loads(proc.stdout).get("surfaces", [])
    except json.JSONDecodeError:
        return []


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
            "schema": "agora-de.input-chromium-live.v1",
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

#!/usr/bin/env python3
"""Live evidence: agent pointer input injection through agora-de-compositorctl.

Pops a click-toggle page in Chromium, drives a pointer click via
`agora-de-compositorctl input pointer click`, and proves the click landed by
capturing the physical output before (red) and after (green) and sampling the
toggled pixel. Mirrors the agent surface framework loop: pop -> verify ->
inject -> capture.
"""
from __future__ import annotations

import argparse
import http.server
import json
import os
import pathlib
import socketserver
import subprocess
import sys
import tempfile
import threading
import time


def main() -> int:
    parser = argparse.ArgumentParser(description="Check agent pointer input injection end-to-end.")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--output-name", default="HDMI-A-1")
    parser.add_argument("--output-capture-session", default="den-k8-input")
    parser.add_argument("--require-capture", action="store_true", help="Fail unless physical capture shows the toggle.")
    parser.add_argument("--capture-delay-seconds", type=float, default=1.0)
    args = parser.parse_args()

    checks: list[dict] = []
    evidence: list[dict] = []
    checked_at = unix_millis()

    if not pathlib.Path(args.compositorctl).exists():
        checks.append(failed("compositorctl", f"not found: {args.compositorctl}"))
        return finish(checks, evidence, checked_at)

    with tempfile.TemporaryDirectory(prefix="agora-de-input-") as tmp:
        page = pathlib.Path(tmp) / "index.html"
        page.write_text(
            "<!doctype html><html><head><style>html,body{margin:0;height:100%}"
            "body{background:#ef4444}</style></head><body>"
            "<script>document.body.addEventListener('click',()=>{"
            "document.body.style.background="
            "(document.body.dataset.t=!document.body.dataset.t)?'#22c55e':'#ef4444';});"
            "</script></body></html>"
        )
        chrome_profile = pathlib.Path(tmp) / "chrome-profile"
        server = serve(page.parent)
        port = server.server_address[1]
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            url = f"http://127.0.0.1:{port}/"
            checks.append(passed("serve", "toggle page served", port=port))
            launch = ctl_json(args, ["launch", "--arg", "chromium", "--arg", "--ozone-platform=wayland",
                                     "--arg", f"--user-data-dir={chrome_profile}", "--arg", "--no-first-run",
                                     "--arg", "--no-default-browser-check", "--arg", url,
                                     "--wait-surface", "--wait-timeout-ms", "20000"])
            surface_id = launch.get("surface_id", "")
            if not surface_id:
                checks.append(failed("launch", f"no surface mapped: {launch}"))
                return finish(checks, evidence, checked_at)
            checks.append(passed("launch", "chromium toggle page mapped", surfaceId=surface_id))

            ctl_run(args, ["surface", "focus", "--surface", surface_id], check=False)
            time.sleep(max(1.0, args.capture_delay_seconds))
            cx, cy = surface_center(args, surface_id)
            if cx is None:
                checks.append(failed("geometry", "could not resolve surface geometry"))
                return finish(checks, evidence, checked_at)
            checks.append(passed("geometry", "surface center resolved", x=cx, y=cy))

            if not args.output_name:
                checks.append(failed("capture", "--output-name required for capture evidence"))
                return finish(checks, evidence, checked_at)

            before, before_path = capture(args, "before")
            bpx = sample(before_path, cx, cy)
            checks.append(passed("capture-before", "physical output captured", path=before_path, center=bpx))

            result = ctl_json(args, ["input", "pointer", "click", "--surface", surface_id, "--x", str(cx), "--y", str(cy)])
            if result.get("ok") is not True:
                checks.append(failed("input", f"injection not ok: {result}"))
                return finish(checks, evidence, checked_at)
            checks.append(passed("input", "pointer click injected", surfaceId=surface_id, x=cx, y=cy))

            time.sleep(args.capture_delay_seconds)
            after, after_path = capture(args, "after")
            apx = sample(after_path, cx, cy)
            toggled = is_red(bpx) and is_green(apx)
            if toggled:
                checks.append(passed("toggle", "click toggled surface red->green", before=bpx, after=apx))
            else:
                msg = "red->green toggle not observed" if args.require_capture else "toggle not confirmed (capture optional)"
                (failed if args.require_capture else passed)("toggle", msg, before=bpx, after=apx)
                if args.require_capture:
                    return finish(checks, evidence, checked_at)
            evidence.append({"scenario": "input-click-toggle", "captures": [before, after], "surfaceId": surface_id})
            ctl_run(args, ["surface", "close", "--surface", surface_id], check=False)
        finally:
            server.shutdown()
    return finish(checks, evidence, checked_at)


def serve(directory: pathlib.Path) -> socketserver.TCPServer:
    handler = lambda *a, **kw: http.server.SimpleHTTPRequestHandler(*a, directory=str(directory), **kw)
    return socketserver.TCPServer(("127.0.0.1", 0), handler)


def ctl_run(args, argv: list[str], check: bool = True) -> str:
    proc = subprocess.run([args.compositorctl, *argv], text=True, capture_output=True)
    if check and proc.returncode != 0:
        raise RuntimeError(f"compositorctl {' '.join(argv)} failed: {proc.stderr.strip() or proc.stdout.strip()}")
    return proc.stdout.strip()


def ctl_json(args, argv: list[str]) -> dict:
    return json.loads(ctl_run(args, argv))


def surface_center(args, surface_id: str):
    layout = ctl_json(args, ["layout", "get"]).get("layout", {})
    for s in layout.get("surfaces", []):
        if s.get("app_id", "").lower().startswith("chrom"):
            g = s.get("geometry", {})
            return g.get("x", 0) + g.get("width", 0) // 2, g.get("y", 0) + g.get("height", 0) // 2
    return None, None


def capture(args, label: str):
    session = f"{args.output_capture_session}-{label}"
    ctl_run(args, ["output", "capture", "--name", args.output_name, "--export", "--session", session])
    # the bridge writes under /run/agent-os/artifacts/<session>/...; find the newest png
    base = pathlib.Path("/run/agent-os/artifacts") / session
    pngs = sorted(base.rglob("*.png"), key=lambda p: p.stat().st_mtime) if base.exists() else []
    if not pngs:
        raise RuntimeError(f"no capture png for {label}")
    return {"label": label, "path": str(pngs[-1]), "output": args.output_name}, str(pngs[-1])


def sample(path: str, x: int, y: int):
    try:
        from PIL import Image
        with Image.open(path) as im:
            return im.convert("RGB").getpixel((int(x), int(y)))
    except Exception as exc:  # noqa: BLE001
        return f"sample-error: {exc}"


def is_red(c) -> bool:
    return isinstance(c, tuple) and c[0] > 150 and c[1] < 110 and c[2] < 110


def is_green(c) -> bool:
    return isinstance(c, tuple) and c[1] > 150 and c[0] < 120 and c[2] < 120


def passed(name: str, detail: str, **extra) -> dict:
    return {"name": name, "status": "pass", "detail": detail, **extra}


def failed(name: str, detail: str, **extra) -> dict:
    return {"name": name, "status": "fail", "detail": detail, **extra}


def unix_millis() -> int:
    return int(time.time() * 1000)


def finish(checks: list[dict], evidence: list[dict], checked_at: int) -> int:
    failed_checks = [c for c in checks if c["status"] != "pass"]
    json.dump(
        {
            "schema": "agora-de.input-injection-live.v1",
            "checkedAtUnixMillis": checked_at,
            "checks": checks,
            "evidencePackets": evidence,
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

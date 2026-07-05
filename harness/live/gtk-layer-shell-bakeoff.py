#!/usr/bin/env python3
import argparse
import json
import os
import pathlib
import signal
import subprocess
import sys
import time
import urllib.parse


DEFAULT_CASES = [
    "gtk3-background",
    "gtk3-panel",
    "gtk4-background",
    "gtk4-panel",
]


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Run GTK3 vs GTK4 WebKit layer-shell presentation bake-off cases."
    )
    parser.add_argument("--helper", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--toolkit", choices=("gtk3", "gtk4"), help=argparse.SUPPRESS)
    parser.add_argument("--role", choices=("background", "panel"), help=argparse.SUPPRESS)
    parser.add_argument("--app-id", help=argparse.SUPPRESS)
    parser.add_argument("--title", help=argparse.SUPPRESS)
    parser.add_argument("--width", type=int, default=2560)
    parser.add_argument("--height", type=int, default=1440)
    parser.add_argument("--python", default="/usr/bin/python3")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--cases", default=",".join(DEFAULT_CASES))
    parser.add_argument("--hold-seconds", type=float, default=4)
    parser.add_argument("--output", default="/tmp/agora-de-gtk-layer-shell-bakeoff.json")
    args = parser.parse_args()

    if args.helper:
        return run_helper(args)

    result = run_bakeoff(args)
    pathlib.Path(args.output).write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 1 if result["summary"]["failed"] else 0


def run_bakeoff(args: argparse.Namespace) -> dict:
    started_at = unix_millis()
    cases = [case.strip() for case in args.cases.split(",") if case.strip()]
    results = []
    for case in cases:
        toolkit, role = parse_case(case)
        results.append(run_case(args, toolkit, role))

    failed = [case for case in results if case["status"] not in {"pass", "insufficient_evidence"}]
    return {
        "schema": "agora-de.gtk-layer-shell-bakeoff.v1",
        "checkedAtUnixMillis": started_at,
        "host": os.uname().nodename,
        "cases": results,
        "summary": {
            "status": "fail" if failed else "pass",
            "passed": len(results) - len(failed),
            "failed": len(failed),
        },
    }


def parse_case(case: str) -> tuple[str, str]:
    parts = case.split("-", 1)
    if len(parts) != 2 or parts[0] not in {"gtk3", "gtk4"} or parts[1] not in {"background", "panel"}:
        raise SystemExit(f"invalid case {case!r}; expected gtk3-background, gtk3-panel, gtk4-background, or gtk4-panel")
    return parts[0], parts[1]


def run_case(args: argparse.Namespace, toolkit: str, role: str) -> dict:
    app_id = f"io.agorade.Bakeoff.{toolkit.upper()}.{role.title()}"
    title = f"AGORA-DE-BAKEOFF-{toolkit.upper()}-{role.upper()}"
    height = 96 if role == "panel" else args.height
    command = [
        args.python,
        str(pathlib.Path(__file__).resolve()),
        "--helper",
        "--toolkit",
        toolkit,
        "--role",
        role,
        "--app-id",
        app_id,
        "--title",
        title,
        "--width",
        str(args.width),
        "--height",
        str(height),
    ]
    started = time.time()
    env = {**os.environ, "PYTHONUNBUFFERED": "1"}
    if toolkit == "gtk4":
        env.setdefault("GDK_BACKEND", "wayland")
        env["LD_PRELOAD"] = prepend_env_path(env.get("LD_PRELOAD", ""), "/usr/lib/libgtk4-layer-shell.so")

    proc = subprocess.Popen(
        command,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env=env,
    )
    try:
        time.sleep(max(0.5, args.hold_seconds))
        readback = read_surface(args.compositorctl, app_id, role)
    finally:
        terminate_process(proc)
    stdout, stderr = proc.communicate(timeout=5)
    duration_ms = int((time.time() - started) * 1000)

    dependency_error = dependency_failure(stdout, stderr)
    if dependency_error:
        status = "dependency_missing"
        detail = dependency_error
    elif readback is None:
        status = "not_mapped"
        detail = "no matching compositor surface was observed"
    else:
        status = "insufficient_evidence"
        if readback.get("role") != role:
            detail = f"surface mapped with effective role {readback.get('role')!r}; physical/capture observation still required"
        else:
            detail = "surface mapped; physical/capture observation still required"

    return {
        "case": f"{toolkit}-{role}",
        "toolkit": toolkit,
        "role": role,
        "appId": app_id,
        "title": title,
        "status": status,
        "detail": detail,
        "durationMillis": duration_ms,
        "surface": readback,
        "stdout": tail_lines(stdout),
        "stderr": tail_lines(stderr),
    }


def dependency_failure(stdout: str, stderr: str) -> str:
    combined = "\n".join([stdout, stderr])
    for line in combined.splitlines():
        if line.startswith("DEPENDENCY_MISSING "):
            return line.removeprefix("DEPENDENCY_MISSING ").strip()
    return ""


def prepend_env_path(existing: str, path: str) -> str:
    parts = [part for part in existing.split(":") if part]
    if path not in parts:
        parts.insert(0, path)
    return ":".join(parts)


def read_surface(compositorctl: str, app_id: str, role: str) -> dict | None:
    completed = subprocess.run(
        [compositorctl, "list-surfaces"],
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=5,
    )
    if completed.returncode != 0:
        return {
            "readbackError": completed.stderr.strip() or f"compositorctl exited {completed.returncode}",
        }
    payload = json.loads(completed.stdout)
    matches = []
    for item in payload.get("surfaces", []):
        surface = item.get("surface") or {}
        if surface.get("app_id") == app_id:
            matches.append(item)
    if not matches:
        return None
    selected = sorted(matches, key=lambda item: item.get("updated_at") or "")[-1]
    surface = selected.get("surface") or {}
    client = selected.get("client") or {}
    layer = surface.get("layer_shell") or {}
    return {
        "id": surface.get("id"),
        "appId": surface.get("app_id"),
        "title": surface.get("title"),
        "role": surface.get("role"),
        "expectedRole": role,
        "roleMatches": surface.get("role") == role,
        "surfaceKind": surface.get("surface_kind"),
        "layer": layer.get("layer"),
        "anchors": layer.get("anchors"),
        "exclusiveZone": layer.get("exclusive_zone"),
        "visible": bool(selected.get("visible") or surface.get("visible")),
        "frameCount": selected.get("frame_count") or 0,
        "outputId": surface.get("output_id") or selected.get("output_id"),
        "geometry": surface.get("geometry") or selected.get("geometry"),
        "pid": client.get("pid"),
    }


def terminate_process(proc: subprocess.Popen) -> None:
    if proc.poll() is not None:
        return
    proc.send_signal(signal.SIGTERM)
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=5)


def tail_lines(text: str, limit: int = 12) -> list[str]:
    lines = [line for line in text.splitlines() if line.strip()]
    return lines[-limit:]


def run_helper(args: argparse.Namespace) -> int:
    if args.toolkit == "gtk3":
        return run_gtk3_helper(args)
    return run_gtk4_helper(args)


def html_document(toolkit: str, role: str) -> str:
    accent = "#00d1b2" if toolkit == "gtk3" else "#ffb020"
    return f"""<!doctype html>
<html>
<head>
  <title>{toolkit} {role} bakeoff</title>
  <style>
    html, body {{
      background: #f8fafc;
      color: #102027;
      height: 100%;
      margin: 0;
      overflow: hidden;
      width: 100%;
    }}
    body {{
      align-items: center;
      box-sizing: border-box;
      display: flex;
      font: 700 22px system-ui, sans-serif;
      gap: 18px;
      padding: 0 28px;
    }}
    .mark {{
      background: {accent};
      border-radius: 4px;
      height: 40px;
      width: 40px;
    }}
  </style>
</head>
<body data-toolkit="{toolkit}" data-role="{role}">
  <span class="mark"></span>
  <span>{toolkit.upper()} WebKit layer-shell {role}</span>
</body>
</html>"""


def data_uri(toolkit: str, role: str) -> str:
    return "data:text/html;charset=utf-8," + urllib.parse.quote(html_document(toolkit, role))


def run_gtk3_helper(args: argparse.Namespace) -> int:
    try:
        import gi

        gi.require_version("Gtk", "3.0")
        gi.require_version("WebKit2", "4.1")
        gi.require_version("GtkLayerShell", "0.1")
        from gi.repository import Gio, GLib, Gtk, GtkLayerShell, WebKit2
    except Exception as exc:
        print(f"DEPENDENCY_MISSING GTK3 stack: {exc}", flush=True)
        return 2

    class App(Gtk.Application):
        def __init__(self) -> None:
            GLib.set_prgname(args.app_id)
            GLib.set_application_name(args.title)
            super().__init__(application_id=args.app_id, flags=Gio.ApplicationFlags.FLAGS_NONE)
            self.window = None

        def do_activate(self) -> None:
            self.window = Gtk.ApplicationWindow(application=self)
            self.window.set_title(args.title)
            self.window.set_default_size(args.width, args.height)
            configure_gtk3_layer_shell(GtkLayerShell, self.window, args.role, args.app_id)
            webview = WebKit2.WebView()
            webview.load_uri(data_uri("gtk3", args.role))
            self.window.add(webview)
            self.window.connect("destroy", lambda *_args: self.quit())
            self.window.show_all()
            print(json.dumps({"event": "shown", "toolkit": "gtk3", "role": args.role, "appId": args.app_id}), flush=True)

    return App().run([])


def configure_gtk3_layer_shell(GtkLayerShell, window, role: str, namespace: str) -> None:
    if not GtkLayerShell.is_supported():
        raise RuntimeError("GtkLayerShell is not supported on this Wayland display")
    GtkLayerShell.init_for_window(window)
    GtkLayerShell.set_namespace(window, namespace)
    if role == "background":
        GtkLayerShell.set_layer(window, GtkLayerShell.Layer.BOTTOM)
        for edge in (GtkLayerShell.Edge.TOP, GtkLayerShell.Edge.BOTTOM, GtkLayerShell.Edge.LEFT, GtkLayerShell.Edge.RIGHT):
            GtkLayerShell.set_anchor(window, edge, True)
    else:
        GtkLayerShell.set_layer(window, GtkLayerShell.Layer.TOP)
        GtkLayerShell.set_anchor(window, GtkLayerShell.Edge.BOTTOM, True)
        GtkLayerShell.auto_exclusive_zone_enable(window)


def run_gtk4_helper(args: argparse.Namespace) -> int:
    try:
        import gi

        gi.require_version("Gtk", "4.0")
        gi.require_version("WebKit", "6.0")
        gi.require_version("Gtk4LayerShell", "1.0")
        from gi.repository import Gio, GLib, Gtk, Gtk4LayerShell, WebKit
    except Exception as exc:
        print(f"DEPENDENCY_MISSING GTK4 stack: {exc}", flush=True)
        return 2

    class App(Gtk.Application):
        def __init__(self) -> None:
            GLib.set_prgname(args.app_id)
            GLib.set_application_name(args.title)
            super().__init__(application_id=args.app_id, flags=Gio.ApplicationFlags.FLAGS_NONE)
            self.window = None

        def do_activate(self) -> None:
            self.window = Gtk.ApplicationWindow(application=self)
            self.window.set_title(args.title)
            self.window.set_default_size(args.width, args.height)
            configure_gtk4_layer_shell(Gtk4LayerShell, self.window, args.role, args.app_id)
            webview = WebKit.WebView()
            webview.load_uri(data_uri("gtk4", args.role))
            self.window.set_child(webview)
            self.window.connect("destroy", lambda *_args: self.quit())
            self.window.present()
            print(json.dumps({"event": "shown", "toolkit": "gtk4", "role": args.role, "appId": args.app_id}), flush=True)

    return App().run([])


def configure_gtk4_layer_shell(Gtk4LayerShell, window, role: str, namespace: str) -> None:
    if not Gtk4LayerShell.is_supported():
        raise RuntimeError("Gtk4LayerShell is not supported on this Wayland display")
    Gtk4LayerShell.init_for_window(window)
    Gtk4LayerShell.set_namespace(window, namespace)
    if role == "background":
        Gtk4LayerShell.set_layer(window, Gtk4LayerShell.Layer.BOTTOM)
        for edge in (Gtk4LayerShell.Edge.TOP, Gtk4LayerShell.Edge.BOTTOM, Gtk4LayerShell.Edge.LEFT, Gtk4LayerShell.Edge.RIGHT):
            Gtk4LayerShell.set_anchor(window, edge, True)
    else:
        Gtk4LayerShell.set_layer(window, Gtk4LayerShell.Layer.TOP)
        Gtk4LayerShell.set_anchor(window, Gtk4LayerShell.Edge.BOTTOM, True)
        Gtk4LayerShell.auto_exclusive_zone_enable(window)


def unix_millis() -> int:
    return int(time.time() * 1000)


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
import argparse
import importlib.util
import json
import pathlib
import subprocess
import sys
import time


SCHEMA = "agora-de.multi-output-workspaces-live.v1"


def main() -> int:
    parser = argparse.ArgumentParser(description="Record installed-service multi-output workspace evidence.")
    parser.add_argument("--base-url", default="http://127.0.0.1:17780")
    parser.add_argument("--compositorctl", default="/home/agent/.local/bin/agora-de-compositorctl")
    parser.add_argument("--capture-session", default="den-k8-multi-output-workspaces")
    parser.add_argument("--timeout-seconds", type=float, default=6.0)
    args = parser.parse_args()

    structured = load_structured_module()
    checked_at = structured.unix_millis()
    checks: list[dict] = []
    evidence: dict = {
        "outputs": [],
        "operatorOutputs": [],
        "workspaces": {},
        "layout": {},
        "capture": {},
        "cleanup": {},
    }

    path_check = structured.check_compositorctl_path(args.compositorctl)
    checks.append(path_check)
    if path_check["status"] != "pass":
        return finish(checked_at, args, checks, evidence)

    before_surface_ids = work_surface_ids(args.base_url)
    evidence["cleanup"]["beforeWorkSurfaceIds"] = sorted(before_surface_ids)

    try:
        output_response = run_compositorctl_json(args.compositorctl, ["output", "list"])
        outputs = [item for item in output_response.get("outputs", []) if isinstance(item, dict)]
        evidence["outputs"] = outputs
        if outputs:
            checks.append(passed("output-discovery", "compositorctl reported physical outputs", outputCount=len(outputs), outputNames=output_names(outputs)))
        else:
            checks.append(failed("output-discovery", "compositorctl output list returned no outputs"))

        operator = structured.get_json(args.base_url + "/api/operator/status")
        operator_outputs = [item for item in operator.get("outputs", []) if isinstance(item, dict)]
        evidence["operatorOutputs"] = operator_outputs
        if operator_outputs:
            checks.append(passed("operator-output-discovery", "/api/operator/status reports outputs", outputCount=len(operator_outputs)))
        else:
            checks.append(failed("operator-output-discovery", "/api/operator/status did not report outputs"))

        workspaces = structured.get_json(args.base_url + "/api/workspaces")
        workspace_items = [item for item in workspaces.get("workspaces", []) if isinstance(item, dict)]
        evidence["workspaces"] = workspaces
        if workspace_items and (workspaces.get("currentWorkspaceId") or any(item.get("active") for item in workspace_items)):
            checks.append(
                passed(
                    "workspace-state-shape",
                    "/api/workspaces reports current workspace state",
                    currentWorkspaceId=workspaces.get("currentWorkspaceId") or "",
                    currentOutputId=workspaces.get("currentOutputId") or "",
                    workspaceCount=len(workspace_items),
                )
            )
        else:
            checks.append(failed("workspace-state-shape", "/api/workspaces did not report usable workspace state", response=workspaces))

        layout_response = structured.get_json(args.base_url + "/api/layout")
        layout = layout_response.get("layout") if isinstance(layout_response, dict) else {}
        if not isinstance(layout, dict):
            layout = {}
        evidence["layout"] = layout
        layout_workspaces = [item for item in layout.get("workspaces", []) if isinstance(item, dict)]
        layout_surfaces = [item for item in layout.get("surfaces", []) if isinstance(item, dict)]
        if layout_workspaces:
            checks.append(
                passed(
                    "layout-workspace-shape",
                    "/api/layout reports workspace projection",
                    workspaceCount=len(layout_workspaces),
                    surfaceCount=len(layout_surfaces),
                    revision=layout.get("revision", 0),
                )
            )
        else:
            checks.append(failed("layout-workspace-shape", "/api/layout did not report workspaces", layout=layout))

        checks.extend(check_workspace_output_links(outputs, workspace_items, layout_workspaces, layout_surfaces))

        activation_check = activate_current_workspace(args.base_url, workspaces, workspace_items)
        checks.append(activation_check)

        if outputs:
            capture_check, capture = capture_first_output(args.compositorctl, outputs[0], args.capture_session)
            evidence["capture"] = capture
            checks.append(capture_check)
        else:
            checks.append(failed("output-targeted-capture", "cannot capture without a discovered output"))

        if len(outputs) > 1:
            checks.append(check_multi_output_assertions(outputs, workspace_items, layout_workspaces, layout_surfaces))
        else:
            checks.append(skipped("multi-output-skip", "host exposes one output; multi-output-only assertions skipped", outputCount=len(outputs)))
    except Exception as error:
        checks.append(failed("multi-output-workspaces", f"live evidence probe failed: {error}"))

    after_surface_ids = work_surface_ids(args.base_url)
    evidence["cleanup"]["afterWorkSurfaceIds"] = sorted(after_surface_ids)
    added = sorted(after_surface_ids - before_surface_ids)
    if added:
        checks.append(failed("cleanup", "live harness left new work surfaces behind", addedSurfaceIds=added))
    else:
        checks.append(passed("cleanup", "live harness did not leave stale work surfaces", workSurfaceCount=len(after_surface_ids)))

    return finish(checked_at, args, checks, evidence)


def load_structured_module():
    path = pathlib.Path(__file__).with_name("check-structured-layout.py")
    spec = importlib.util.spec_from_file_location("agora_de_check_structured_layout", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load structured layout module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def run_compositorctl_json(compositorctl: str, args: list[str]) -> dict:
    completed = subprocess.run([compositorctl, "--pretty", *args], check=False, text=True, capture_output=True, timeout=10)
    if completed.returncode != 0:
        detail = completed.stderr.strip() or completed.stdout.strip()
        raise RuntimeError(f"compositorctl {' '.join(args)} failed: {detail}")
    try:
        return json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        raise RuntimeError(f"compositorctl {' '.join(args)} returned invalid JSON: {completed.stdout}") from error


def work_surface_ids(base_url: str) -> set[str]:
    try:
        surfaces = load_structured_module().get_json(base_url + "/api/surfaces").get("surfaces", [])
    except Exception:
        return set()
    ids = set()
    for surface in surfaces:
        if not isinstance(surface, dict) or not surface.get("mapped"):
            continue
        if surface.get("surfaceKind") == "layer_shell":
            continue
        surface_id = surface.get("id")
        if surface_id:
            ids.add(surface_id)
    return ids


def output_names(outputs: list[dict]) -> list[str]:
    return [str(item.get("name") or "") for item in outputs if item.get("name")]


def check_workspace_output_links(outputs: list[dict], workspaces: list[dict], layout_workspaces: list[dict], layout_surfaces: list[dict]) -> list[dict]:
    checks = []
    workspace_output_ids = {str(item.get("outputId") or item.get("output_id") or "") for item in workspaces + layout_workspaces}
    workspace_output_ids.discard("")
    surface_output_ids = {str(item.get("outputId") or item.get("output_id") or "") for item in layout_surfaces}
    surface_output_ids.discard("")
    if workspace_output_ids or surface_output_ids:
        checks.append(
            passed(
                "workspace-output-links",
                "workspace/layout projection carries output identity when known",
                workspaceOutputIds=sorted(workspace_output_ids),
                surfaceOutputIds=sorted(surface_output_ids),
            )
        )
    elif len(outputs) <= 1:
        checks.append(skipped("workspace-output-links", "single-output host has no required per-workspace output labels", outputCount=len(outputs)))
    else:
        checks.append(failed("workspace-output-links", "multi-output host did not expose output identity on workspace or surface state"))
    return checks


def activate_current_workspace(base_url: str, workspace_response: dict, workspaces: list[dict]) -> dict:
    current_id = str(workspace_response.get("currentWorkspaceId") or "")
    current = next((item for item in workspaces if item.get("id") == current_id), None)
    if current is None:
        current = next((item for item in workspaces if item.get("active")), None) or (workspaces[0] if workspaces else {})
        current_id = str(current.get("id") or "workspace-1")
    body = {"workspaceId": current_id, "action": "activate"}
    output_id = str(current.get("outputId") or workspace_response.get("currentOutputId") or "")
    if output_id:
        body["outputId"] = output_id
    try:
        result = load_structured_module().post_json(base_url + "/api/workspaces/action", body)
    except Exception as error:
        return failed("workspace-activation", f"/api/workspaces/action failed: {error}", body=body)
    if result.get("status") != "accepted":
        return failed("workspace-activation", "workspace activation did not return accepted", body=body, response=result)
    return passed("workspace-activation", "workspace activation accepted for reported current workspace", body=body, currentWorkspaceId=result.get("currentWorkspaceId") or "")


def capture_first_output(compositorctl: str, output: dict, session_id: str) -> tuple[dict, dict]:
    name = str(output.get("name") or "")
    if not name:
        return failed("output-targeted-capture", "first output has no name"), {}
    try:
        capture = run_compositorctl_json(compositorctl, ["output", "capture", "--name", name, "--session", session_id])
    except Exception as error:
        return failed("output-targeted-capture", f"output capture failed: {error}", outputName=name), {}
    captures = capture.get("captures") or []
    if capture.get("output") == name and isinstance(captures, list):
        return passed("output-targeted-capture", "compositorctl captured the targeted output", outputName=name, captureCount=len(captures)), capture
    return failed("output-targeted-capture", "output capture response did not match target output", outputName=name, response=capture), capture


def check_multi_output_assertions(outputs: list[dict], workspaces: list[dict], layout_workspaces: list[dict], layout_surfaces: list[dict]) -> dict:
    output_ids = set(output_names(outputs))
    workspace_output_ids = {str(item.get("outputId") or item.get("output_id") or "") for item in workspaces + layout_workspaces}
    workspace_output_ids.discard("")
    surface_output_ids = {str(item.get("outputId") or item.get("output_id") or "") for item in layout_surfaces}
    surface_output_ids.discard("")
    if workspace_output_ids.intersection(output_ids) or surface_output_ids.intersection(output_ids):
        return passed(
            "multi-output-assertions",
            "multi-output host exposes targetable output/workspace/surface identity",
            outputNames=sorted(output_ids),
            workspaceOutputIds=sorted(workspace_output_ids),
            surfaceOutputIds=sorted(surface_output_ids),
        )
    return failed(
        "multi-output-assertions",
        "multi-output host lacks output ids on workspace/surface state",
        outputNames=sorted(output_ids),
        workspaceOutputIds=sorted(workspace_output_ids),
        surfaceOutputIds=sorted(surface_output_ids),
    )


def passed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "pass", "detail": detail, **extra}


def skipped(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "skip", "detail": detail, **extra}


def failed(name: str, detail: str, **extra: object) -> dict:
    return {"name": name, "status": "fail", "detail": detail, **extra}


def finish(checked_at: int, args: argparse.Namespace, checks: list[dict], evidence: dict) -> int:
    failed_checks = [check for check in checks if check.get("status") == "fail"]
    result = {
        "schema": SCHEMA,
        "checkedAtUnixMillis": checked_at,
        "baseUrl": args.base_url,
        "compositorctl": args.compositorctl,
        "checks": checks,
        "evidence": evidence,
        "summary": {
            "status": "fail" if failed_checks else "pass",
            "passed": len([check for check in checks if check.get("status") == "pass"]),
            "skipped": len([check for check in checks if check.get("status") == "skip"]),
            "failed": len(failed_checks),
        },
    }
    print(json.dumps(result, indent=2, sort_keys=True))
    return 1 if failed_checks else 0


if __name__ == "__main__":
    raise SystemExit(main())

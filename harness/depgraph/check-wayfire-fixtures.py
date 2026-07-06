#!/usr/bin/env python3
import json
import pathlib
import sys


KNOWN_TYPES = {
    "surface_event",
    "layout_state",
    "policy_replace",
    "policy_upsert",
    "policy_remove",
    "input_context",
    "close_surface",
    "close_surfaces_by_uid",
    "place_surface",
    "place_response",
}

PROBE_SCHEMA = "agora-de.wayfire-layout-authority-probe.v1"


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    fixture_dir = root / "compositor" / "protocol-fixtures" / "wayfire"
    failures: list[str] = []

    for path in sorted(fixture_dir.glob("*.jsonl")):
        line_count = 0
        for line_number, line in enumerate(path.read_text().splitlines(), start=1):
            if not line.strip():
                continue
            line_count += 1
            try:
                payload = json.loads(line)
            except json.JSONDecodeError as error:
                failures.append(f"{path.relative_to(root)}:{line_number}: invalid JSON: {error}")
                continue
            message_type = payload.get("type")
            if message_type not in KNOWN_TYPES:
                failures.append(f"{path.relative_to(root)}:{line_number}: unknown message type {message_type!r}")
        if line_count == 0:
            failures.append(f"{path.relative_to(root)} must not be empty")

    required = {"plugin-events.jsonl", "bridge-commands.jsonl", "layout-state-events.jsonl"}
    present = {path.name for path in fixture_dir.glob("*.jsonl")}
    missing = required - present
    for name in sorted(missing):
        failures.append(f"missing Wayfire protocol fixture: {name}")

    probe_path = fixture_dir / "layout-authority-probe-4267.json"
    if not probe_path.exists():
        failures.append("missing Wayfire layout-authority probe fixture: layout-authority-probe-4267.json")
    else:
        try:
            probe = json.loads(probe_path.read_text())
        except json.JSONDecodeError as error:
            failures.append(f"{probe_path.relative_to(root)}: invalid JSON: {error}")
        else:
            if probe.get("schema") != PROBE_SCHEMA:
                failures.append(f"{probe_path.relative_to(root)}: unexpected schema {probe.get('schema')!r}")
            if probe.get("taskId") != 4267:
                failures.append(f"{probe_path.relative_to(root)}: taskId must be 4267")
            if not probe.get("wayfire", {}).get("pkgConfigVersion"):
                failures.append(f"{probe_path.relative_to(root)}: missing wayfire.pkgConfigVersion")
            requirements = probe.get("requirements")
            if not isinstance(requirements, list) or not requirements:
                failures.append(f"{probe_path.relative_to(root)}: requirements must be a non-empty list")
            else:
                requirement_names = {entry.get("name") for entry in requirements if isinstance(entry, dict)}
                for name in ["post_layout_geometry", "workspace_state", "focus_state", "zone_assignment"]:
                    if name not in requirement_names:
                        failures.append(f"{probe_path.relative_to(root)}: missing requirement {name!r}")
            decision = probe.get("decision", {})
            if decision.get("wayfireProofViable") is not True:
                failures.append(f"{probe_path.relative_to(root)}: wayfireProofViable must be true for the 4268 proof path")
            if decision.get("nextTask") != 4268:
                failures.append(f"{probe_path.relative_to(root)}: nextTask must be 4268")

    if failures:
        print("\n".join(failures))
        return 1

    print("Wayfire protocol fixtures: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

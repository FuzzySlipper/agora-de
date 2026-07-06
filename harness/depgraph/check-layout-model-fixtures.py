#!/usr/bin/env python3
import json
import pathlib
import sys


SCHEMA = "agora-de.layout-model.command-semantics.v1"
KNOWN_ACTIONS = {
    "layout.set_mode",
    "surface.move_resize",
    "surface.tile",
    "surface.set_floating",
    "surface.assign_zone",
    "surface.maximize",
    "surface.minimize",
    "surface.fullscreen",
    "workspace.activate",
}
KNOWN_STATUSES = {"accepted", "rejected"}
KNOWN_ERRORS = {"invalid_request", "surface_not_found", "surface_stale", "backend_unsupported"}
KNOWN_PARTICIPATION = {"tiled", "floating", "transient"}


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    fixture_dir = root / "compositor" / "protocol-fixtures" / "layout-model"
    fixture_path = fixture_dir / "command-semantics.json"
    failures: list[str] = []

    if not fixture_path.exists():
        print("missing layout model fixture: command-semantics.json")
        return 1
    fixture = json.loads(fixture_path.read_text(encoding="utf-8"))
    validate_fixture(fixture, failures)

    if failures:
        print("\n".join(failures))
        return 1
    print("layout model fixtures: OK")
    return 0


def validate_fixture(fixture: dict, failures: list[str]) -> None:
    if fixture.get("schema") != SCHEMA:
        failures.append("command-semantics.json: unexpected schema")

    initial = fixture.get("initial")
    if not isinstance(initial, dict):
        failures.append("command-semantics.json: initial must be an object")
        return
    surfaces = initial.get("surfaces")
    if not isinstance(surfaces, list) or not surfaces:
        failures.append("command-semantics.json: initial.surfaces must be non-empty")
    else:
        seen = set()
        for surface in surfaces:
            surface_id = surface.get("surface_id")
            if not isinstance(surface_id, str) or not surface_id:
                failures.append("command-semantics.json: surface_id is required")
                continue
            if surface_id in seen:
                failures.append(f"command-semantics.json: duplicate surface {surface_id}")
            seen.add(surface_id)
            validate_geometry(surface.get("geometry"), f"surface {surface_id}", failures)
            if surface.get("participation") not in KNOWN_PARTICIPATION:
                failures.append(f"command-semantics.json: invalid participation for {surface_id}")

    commands = fixture.get("commands")
    if not isinstance(commands, list) or not commands:
        failures.append("command-semantics.json: commands must be non-empty")
    else:
        accepted_count = 0
        rejected_count = 0
        for index, command in enumerate(commands):
            prefix = f"command {index}"
            action = command.get("action")
            if action not in KNOWN_ACTIONS:
                failures.append(f"{prefix}: unknown action {action!r}")
            if "backend_geometry" in command:
                validate_geometry(command.get("backend_geometry"), prefix, failures)
            expect = command.get("expect")
            if not isinstance(expect, dict):
                failures.append(f"{prefix}: expect must be an object")
                continue
            status = expect.get("status")
            if status not in KNOWN_STATUSES:
                failures.append(f"{prefix}: invalid status {status!r}")
            if status == "accepted":
                accepted_count += 1
                if expect.get("participation") and expect["participation"] not in KNOWN_PARTICIPATION:
                    failures.append(f"{prefix}: invalid expected participation")
            if status == "rejected":
                rejected_count += 1
                if expect.get("error_class") not in KNOWN_ERRORS:
                    failures.append(f"{prefix}: rejected command missing stable error_class")
        if accepted_count == 0 or rejected_count == 0:
            failures.append("command-semantics.json: fixture must include accepted and rejected commands")

    final = fixture.get("final")
    if not isinstance(final, dict):
        failures.append("command-semantics.json: final must be an object")
        return
    for key in ["surface_order", "focus_order"]:
        if not isinstance(final.get(key), list) or not final[key]:
            failures.append(f"command-semantics.json: final.{key} must be non-empty")
    if not isinstance(final.get("zones"), dict) or not final["zones"]:
        failures.append("command-semantics.json: final.zones must be non-empty")


def validate_geometry(value: object, prefix: str, failures: list[str]) -> None:
    if not isinstance(value, dict):
        failures.append(f"{prefix}: geometry must be an object")
        return
    for key in ["x", "y", "width", "height"]:
        if not isinstance(value.get(key), int):
            failures.append(f"{prefix}: geometry.{key} must be an integer")
    if isinstance(value.get("width"), int) and value["width"] <= 0:
        failures.append(f"{prefix}: geometry.width must be positive")
    if isinstance(value.get("height"), int) and value["height"] <= 0:
        failures.append(f"{prefix}: geometry.height must be positive")


if __name__ == "__main__":
    raise SystemExit(main())

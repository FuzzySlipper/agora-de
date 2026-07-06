#!/usr/bin/env python3
import json
import pathlib
import sys


SCHEMA = "agora-de.layout-model.command-semantics.v1"
PLANNER_SCHEMA = "agora-de.layout-model.planner-input-output.v1"
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
KNOWN_RULES = {"zones", "master_stack", "dwindle"}
KNOWN_MODES = {"freeform", "zones", "columns"}


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    fixture_dir = root / "compositor" / "protocol-fixtures" / "layout-model"
    fixture_path = fixture_dir / "command-semantics.json"
    planner_fixture_path = fixture_dir / "planner-input-output.json"
    failures: list[str] = []

    if not fixture_path.exists():
        print("missing layout model fixture: command-semantics.json")
        return 1
    fixture = json.loads(fixture_path.read_text(encoding="utf-8"))
    validate_fixture(fixture, failures)
    if not planner_fixture_path.exists():
        failures.append("missing layout model fixture: planner-input-output.json")
    else:
        planner_fixture = json.loads(planner_fixture_path.read_text(encoding="utf-8"))
        validate_planner_fixture(planner_fixture, failures)

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


def validate_planner_fixture(fixture: dict, failures: list[str]) -> None:
    if fixture.get("schema") != PLANNER_SCHEMA:
        failures.append("planner-input-output.json: unexpected schema")
    description = fixture.get("description")
    if not isinstance(description, str) or "Backend acknowledgement geometry" not in description:
        failures.append("planner-input-output.json: description must distinguish backend acknowledgement")

    boundary = fixture.get("boundary")
    if not isinstance(boundary, dict):
        failures.append("planner-input-output.json: boundary must be an object")
        return
    for key in ["planner_input", "planner_output", "backend_acknowledgement"]:
        values = boundary.get(key)
        if not isinstance(values, list) or not values:
            failures.append(f"planner-input-output.json: boundary.{key} must be non-empty")
    if "desired rectangles" not in boundary.get("planner_output", []):
        failures.append("planner-input-output.json: planner_output must include desired rectangles")
    if "post-placement geometry" not in boundary.get("backend_acknowledgement", []):
        failures.append("planner-input-output.json: backend_acknowledgement must include post-placement geometry")

    planner_input = fixture.get("input")
    if not isinstance(planner_input, dict):
        failures.append("planner-input-output.json: input must be an object")
        return
    if planner_input.get("rule") not in KNOWN_RULES:
        failures.append("planner-input-output.json: input.rule is unknown")
    if not isinstance(planner_input.get("revision"), int):
        failures.append("planner-input-output.json: input.revision must be an integer")
    validate_geometry(planner_input.get("output"), "planner input output", failures)
    validate_reserved_chrome(planner_input.get("reserved_chrome"), failures)
    validate_settings(planner_input.get("settings"), failures)
    validate_string_list(planner_input.get("focus_order"), "planner input focus_order", failures)

    surfaces = planner_input.get("surfaces")
    if not isinstance(surfaces, list) or len(surfaces) < 2:
        failures.append("planner-input-output.json: input.surfaces must include at least two surfaces")
    else:
        seen_orders = set()
        for surface in surfaces:
            surface_id = surface.get("surface_id")
            if not isinstance(surface_id, str) or not surface_id:
                failures.append("planner-input-output.json: input surface_id is required")
            if surface.get("participation") not in KNOWN_PARTICIPATION:
                failures.append(f"planner-input-output.json: invalid participation for {surface_id}")
            order = surface.get("order")
            if not isinstance(order, int) or order < 0:
                failures.append(f"planner-input-output.json: invalid order for {surface_id}")
            if order in seen_orders:
                failures.append(f"planner-input-output.json: duplicate order {order}")
            seen_orders.add(order)

    expected = fixture.get("expected_plan")
    if not isinstance(expected, dict):
        failures.append("planner-input-output.json: expected_plan must be an object")
        return
    if expected.get("rule") not in KNOWN_RULES:
        failures.append("planner-input-output.json: expected_plan.rule is unknown")
    if expected.get("mode") not in KNOWN_MODES:
        failures.append("planner-input-output.json: expected_plan.mode is unknown")
    if not isinstance(expected.get("revision"), int):
        failures.append("planner-input-output.json: expected_plan.revision must be an integer")
    validate_string_list(expected.get("surface_order"), "expected_plan surface_order", failures)
    validate_string_list(expected.get("focus_order"), "expected_plan focus_order", failures)
    planned_surfaces = expected.get("surfaces")
    if not isinstance(planned_surfaces, list) or len(planned_surfaces) < 2:
        failures.append("planner-input-output.json: expected_plan.surfaces must include at least two surfaces")
    else:
        zones = set()
        for surface in planned_surfaces:
            surface_id = surface.get("surface_id")
            if not isinstance(surface_id, str) or not surface_id:
                failures.append("planner-input-output.json: expected surface_id is required")
            zone_id = surface.get("zone_id")
            if not isinstance(zone_id, str) or not zone_id:
                failures.append(f"planner-input-output.json: missing zone_id for {surface_id}")
            else:
                zones.add(zone_id)
            if surface.get("participation") not in KNOWN_PARTICIPATION:
                failures.append(f"planner-input-output.json: invalid planned participation for {surface_id}")
            validate_geometry(surface.get("desired_geometry"), f"planned surface {surface_id}", failures)
        if not {"primary", "secondary"}.issubset(zones):
            failures.append("planner-input-output.json: expected_plan must include primary and secondary zones")


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


def validate_reserved_chrome(value: object, failures: list[str]) -> None:
    if not isinstance(value, dict):
        failures.append("planner-input-output.json: reserved_chrome must be an object")
        return
    for key in ["top", "right", "bottom", "left"]:
        if not isinstance(value.get(key), int) or value[key] < 0:
            failures.append(f"planner-input-output.json: reserved_chrome.{key} must be a non-negative integer")


def validate_settings(value: object, failures: list[str]) -> None:
    if not isinstance(value, dict):
        failures.append("planner-input-output.json: settings must be an object")
        return
    gaps = value.get("gaps")
    if not isinstance(gaps, dict):
        failures.append("planner-input-output.json: settings.gaps must be an object")
    else:
        for key in ["outer_horizontal", "outer_vertical", "inner_horizontal", "inner_vertical"]:
            if not isinstance(gaps.get(key), int) or gaps[key] < 0:
                failures.append(f"planner-input-output.json: gaps.{key} must be a non-negative integer")
    if not isinstance(value.get("nmaster"), int) or value["nmaster"] < 0:
        failures.append("planner-input-output.json: settings.nmaster must be a non-negative integer")
    if not isinstance(value.get("mfact"), (int, float)):
        failures.append("planner-input-output.json: settings.mfact must be numeric")
    if not isinstance(value.get("smart_gaps"), bool):
        failures.append("planner-input-output.json: settings.smart_gaps must be boolean")


def validate_string_list(value: object, prefix: str, failures: list[str]) -> None:
    if not isinstance(value, list) or not value:
        failures.append(f"planner-input-output.json: {prefix} must be a non-empty list")
        return
    for item in value:
        if not isinstance(item, str) or not item:
            failures.append(f"planner-input-output.json: {prefix} items must be non-empty strings")


if __name__ == "__main__":
    raise SystemExit(main())

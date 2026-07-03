#!/usr/bin/env python3
import json
import pathlib
import sys


KNOWN_TYPES = {
    "surface_event",
    "policy_replace",
    "policy_upsert",
    "policy_remove",
    "input_context",
    "close_surface",
    "close_surfaces_by_uid",
}


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

    required = {"plugin-events.jsonl", "bridge-commands.jsonl"}
    present = {path.name for path in fixture_dir.glob("*.jsonl")}
    missing = required - present
    for name in sorted(missing):
        failures.append(f"missing Wayfire protocol fixture: {name}")

    if failures:
        print("\n".join(failures))
        return 1

    print("Wayfire protocol fixtures: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())


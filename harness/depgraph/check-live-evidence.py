#!/usr/bin/env python3
import json
import pathlib
import sys


VISUAL_STATUSES = {"visible", "blank", "unknown"}
CLASSIFICATIONS = {
    "insufficient_mapped_only",
    "content_committed",
    "frame_presented",
    "capture_visible",
    "blank_capture_failure",
    "not_visible",
}
EXPECTED_FIXTURES = {
    "mapped-only.json": ("unknown", "insufficient_mapped_only", "pass"),
    "visible-capture.json": ("visible", "capture_visible", "pass"),
    "blank-capture.json": ("blank", "blank_capture_failure", "fail"),
    "not-visible.json": ("unknown", "not_visible", "fail"),
}


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    fixtures = root / "harness" / "fixtures" / "live-evidence"
    failures: list[str] = []

    for name, expected in EXPECTED_FIXTURES.items():
        path = fixtures / name
        if not path.exists():
            failures.append(f"missing live evidence fixture: {name}")
            continue
        validate_fixture(name, json.loads(path.read_text()), expected, failures)

    if failures:
        print("\n".join(failures))
        return 1

    print("live evidence fixtures: OK")
    return 0


def validate_fixture(name: str, fixture: dict, expected: tuple[str, str, str], failures: list[str]) -> None:
    expected_visual, expected_classification, expected_summary = expected
    if fixture.get("schema") != "agora-de.den-k8-live-evidence.v1":
        failures.append(f"{name}: unexpected schema")
    if not isinstance(fixture.get("checkedAtUnixMillis"), int):
        failures.append(f"{name}: checkedAtUnixMillis must be integer")

    checks = fixture.get("checks")
    if not isinstance(checks, list) or not checks:
        failures.append(f"{name}: checks must be non-empty")
    else:
        failed = len([check for check in checks if check.get("status") != "pass"])
        summary = fixture.get("summary", {})
        if summary.get("status") != expected_summary:
            failures.append(f"{name}: summary status {summary.get('status')} != {expected_summary}")
        if summary.get("failed") != failed:
            failures.append(f"{name}: summary failed count does not match checks")

    packets = fixture.get("evidencePackets")
    if not isinstance(packets, list) or not packets:
        failures.append(f"{name}: evidencePackets must be non-empty")
        return

    packet = packets[-1]
    if not isinstance(packet.get("scenario"), str) or not packet["scenario"].strip():
        failures.append(f"{name}: packet scenario is required")
    if not isinstance(packet.get("capturedAtUnixMillis"), int):
        failures.append(f"{name}: packet timestamp must be integer")
    if packet.get("visualStatus") not in VISUAL_STATUSES:
        failures.append(f"{name}: invalid visualStatus {packet.get('visualStatus')}")
    if packet.get("captureClassification") not in CLASSIFICATIONS:
        failures.append(f"{name}: invalid captureClassification {packet.get('captureClassification')}")
    if packet.get("visualStatus") != expected_visual:
        failures.append(f"{name}: visualStatus {packet.get('visualStatus')} != {expected_visual}")
    if packet.get("captureClassification") != expected_classification:
        failures.append(
            f"{name}: captureClassification {packet.get('captureClassification')} != {expected_classification}"
        )

    if packet.get("captureClassification") == "blank_capture_failure" and fixture.get("summary", {}).get("status") != "fail":
        failures.append(f"{name}: blank capture must fail the run")
    if packet.get("captureClassification") == "insufficient_mapped_only" and packet.get("visualStatus") != "unknown":
        failures.append(f"{name}: mapped-only evidence cannot claim visibility")


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
import json
import pathlib
import sys


SCHEMA = "agora-de.smithay-layout-authority-spike.v1"
REQUIRED_DEN_4251 = {
    "authoritative post-layout geometry",
    "commandable layout actions",
    "stable workspace and focus order",
    "cleanup and stale classification",
    "capture-visible annotations",
}


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    path = root / "compositor" / "protocol-fixtures" / "smithay" / "layout-authority-spike-4271.json"
    failures: list[str] = []

    if not path.exists():
        print("missing Smithay spike fixture: layout-authority-spike-4271.json")
        return 1

    fixture = json.loads(path.read_text(encoding="utf-8"))
    if fixture.get("schema") != SCHEMA:
        failures.append("Smithay spike fixture has unexpected schema")
    if fixture.get("taskId") != 4271:
        failures.append("Smithay spike fixture taskId must be 4271")
    if fixture.get("status") != "parked":
        failures.append("Smithay spike must remain parked until Wayfire proof evidence changes")
    if not isinstance(fixture.get("parkReason"), str) or "Wayfire" not in fixture["parkReason"]:
        failures.append("Smithay spike must explain why it is parked")

    sources = fixture.get("primarySources")
    if not isinstance(sources, list) or len(sources) < 3:
        failures.append("Smithay spike fixture must include primary source references")
    else:
        for source in sources:
            for field in ("name", "url", "finding"):
                if not isinstance(source.get(field), str) or not source[field].strip():
                    failures.append(f"Smithay primary source missing {field}")

    trigger = fixture.get("implementationTrigger", {})
    start_only_if = trigger.get("startOnlyIf")
    if not isinstance(start_only_if, list) or len(start_only_if) < 5:
        failures.append("Smithay spike must list Den 4251 trigger conditions")
    current_evidence = trigger.get("currentEvidence")
    if not isinstance(current_evidence, list) or not any("4269" in item for item in current_evidence):
        failures.append("Smithay spike must cite current Wayfire command proof evidence")

    deliverables = fixture.get("minimalDeliverables")
    if not isinstance(deliverables, list) or len(deliverables) < 4:
        failures.append("Smithay spike must list minimal deliverables")
    else:
        ids = {item.get("id") for item in deliverables if isinstance(item, dict)}
        for required in ["host-native-clients", "layout-contract", "focus-order-cleanup", "capture-visible-evidence"]:
            if required not in ids:
                failures.append(f"Smithay spike missing deliverable {required}")

    non_goals = fixture.get("nonGoals")
    if not isinstance(non_goals, list) or "shell UI" not in non_goals or "governance or audit log integration" not in non_goals:
        failures.append("Smithay spike must explicitly exclude shell and governance work")

    mapped = {entry.get("criterion") for entry in fixture.get("den4251Criteria", []) if isinstance(entry, dict)}
    missing = REQUIRED_DEN_4251 - mapped
    for criterion in sorted(missing):
        failures.append(f"Smithay spike missing Den 4251 mapping: {criterion}")

    validation = fixture.get("validation", {})
    if not validation.get("local") or not validation.get("live"):
        failures.append("Smithay spike must define local and live validation")

    stop_go = fixture.get("stopGoCriteria", {})
    if not stop_go.get("go") or not stop_go.get("stop"):
        failures.append("Smithay spike must define stop/go criteria")

    if failures:
        print("\n".join(failures))
        return 1

    print("Smithay spike fixture: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
import json
import pathlib
import sys


SCHEMA = "agora-de.layout-authority-closeout.v1"
REQUIRED_EVIDENCE_TASKS = {4267, 4268, 4269, 4270, 4271}
REQUIRED_RUST_OWNS = {
    "layout rule selection and state transitions",
    "surface ordering, focus ordering, revisions, stale cleanup, and classified command results",
}
REQUIRED_WAYFIRE_OWNS = {
    "native surface placement execution",
    "post-placement geometry acknowledgement",
}


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    path = root / "compositor" / "protocol-fixtures" / "layout-authority-closeout-4272.json"
    failures: list[str] = []

    if not path.exists():
        print("missing layout authority closeout fixture")
        return 1

    fixture = json.loads(path.read_text(encoding="utf-8"))
    if fixture.get("schema") != SCHEMA:
        failures.append("layout authority closeout fixture has unexpected schema")
    if fixture.get("taskId") != 4272:
        failures.append("layout authority closeout taskId must be 4272")
    if fixture.get("parentTaskId") != 4266:
        failures.append("layout authority closeout parentTaskId must be 4266")
    if fixture.get("decision") != "wayfire-current-backend-rust-layout-planner-authority":
        failures.append("layout authority closeout must record the selected backend boundary")

    evidence_tasks = {
        item.get("taskId")
        for item in fixture.get("evidence", [])
        if isinstance(item, dict)
    }
    missing_evidence = REQUIRED_EVIDENCE_TASKS - evidence_tasks
    for task_id in sorted(missing_evidence):
        failures.append(f"layout authority closeout missing evidence for task {task_id}")

    churn = fixture.get("churnBudget", {})
    if churn.get("status") != "within-budget":
        failures.append("layout authority closeout must explicitly evaluate churn budget")
    if "synthetic keyboard" not in churn.get("result", ""):
        failures.append("layout authority closeout must reject synthetic input dependency")

    lessons = fixture.get("mangoLessons")
    if not isinstance(lessons, list) or len(lessons) < 3:
        failures.append("layout authority closeout must record Mango layout lessons")

    parent = fixture.get("nextImplementationParent", {})
    if parent.get("taskId") != 4318:
        failures.append("layout authority closeout must reference Den parent 4318")
    subtasks = {
        item.get("taskId")
        for item in parent.get("subtasks", [])
        if isinstance(item, dict)
    }
    for task_id in [4319, 4320, 4321, 4322, 4323, 4324]:
        if task_id not in subtasks:
            failures.append(f"layout authority closeout missing follow-up task {task_id}")

    boundary = fixture.get("backendBoundary", {})
    rust_owns = set(boundary.get("rustOwns", []))
    wayfire_owns = set(boundary.get("wayfireOwns", []))
    for item in sorted(REQUIRED_RUST_OWNS - rust_owns):
        failures.append(f"layout authority closeout missing Rust boundary: {item}")
    for item in sorted(REQUIRED_WAYFIRE_OWNS - wayfire_owns):
        failures.append(f"layout authority closeout missing Wayfire boundary: {item}")

    criteria = fixture.get("closeoutCriteria", {})
    for key in [
        "wayfireProofSatisfied",
        "rustContractHardened",
        "smithaySpikeScopedAndParked",
        "docsUpdated",
        "followUpTasksCreated",
    ]:
        if criteria.get(key) is not True:
            failures.append(f"layout authority closeout criterion is not true: {key}")

    docs = [
        root / "docs" / "compositor-backend-decision.md",
        root / "docs" / "compositor-backend-plan.md",
        root / "docs" / "structured-window-handling.md",
    ]
    for doc in docs:
        text = doc.read_text(encoding="utf-8")
        if "backend-neutral Rust layout planner" not in text:
            failures.append(f"{doc.relative_to(root)} must mention backend-neutral Rust layout planner")

    if failures:
        print("\n".join(failures))
        return 1

    print("layout authority closeout fixture: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

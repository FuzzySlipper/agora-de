#!/usr/bin/env python3
import json
import pathlib
import sys


ALLOWED_SUPPORT = {"native", "standard_protocol", "custom_plugin", "missing"}


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    path = root / "compositor" / "protocol-fixtures" / "capabilities" / "backend-capability-matrix.json"
    matrix = json.loads(path.read_text())
    failures: list[str] = []

    if matrix.get("schema") != "agora-de.backend-capability-matrix.v1":
        failures.append("capability matrix has unexpected schema")

    required = matrix.get("requiredCapabilities", [])
    if not required:
        failures.append("capability matrix must list requiredCapabilities")

    backend_ids = set()
    for backend in matrix.get("backends", []):
        backend_id = backend.get("id", "")
        if not backend_id:
            failures.append("backend entry is missing id")
            continue
        if backend_id in backend_ids:
            failures.append(f"duplicate backend id: {backend_id}")
        backend_ids.add(backend_id)

        capabilities = backend.get("capabilities", {})
        missing = [capability for capability in required if capability not in capabilities]
        if missing:
            failures.append(f"{backend_id} missing required capability entries: {', '.join(missing)}")

        for capability, entry in capabilities.items():
            if capability not in required:
                failures.append(f"{backend_id} has unknown capability: {capability}")
            support = entry.get("support")
            evidence = entry.get("evidence")
            if support not in ALLOWED_SUPPORT:
                failures.append(f"{backend_id}.{capability} has invalid support: {support}")
            if not isinstance(evidence, str) or not evidence.strip():
                failures.append(f"{backend_id}.{capability} must include evidence")

    if "wayfire-plugin" not in backend_ids:
        failures.append("capability matrix must include wayfire-plugin")
    if "standard-wayland-protocol-probe" not in backend_ids:
        failures.append("capability matrix must include standard-wayland-protocol-probe")

    standard = next(
        (backend for backend in matrix.get("backends", []) if backend.get("id") == "standard-wayland-protocol-probe"),
        None,
    )
    if standard:
        support = standard.get("capabilities", {}).get("synchronous_input_deny", {}).get("support")
        if support != "missing":
            failures.append("standard-wayland-protocol-probe must keep synchronous_input_deny marked missing until proven otherwise")

    if failures:
        print("\n".join(failures))
        return 1

    print("compositor capability fixtures: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

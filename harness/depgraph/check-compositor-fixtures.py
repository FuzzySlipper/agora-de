#!/usr/bin/env python3
import json
import pathlib
import sys


ALLOWED_SUPPORT = {"native", "standard_protocol", "custom_plugin", "prototype", "missing"}


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    path = root / "compositor" / "protocol-fixtures" / "capabilities" / "backend-capability-matrix.json"
    matrix = json.loads(path.read_text())
    probe_path = root / "compositor" / "standard-protocol-probe" / "probe-observations.json"
    probe = json.loads(probe_path.read_text())
    failures: list[str] = []

    if matrix.get("schema") != "agora-de.backend-capability-matrix.v1":
        failures.append("capability matrix has unexpected schema")
    if matrix.get("updatedForTaskId") != 4572:
        failures.append("capability matrix must record post-northstar task 4572 update")

    required = matrix.get("requiredCapabilities", [])
    if not required:
        failures.append("capability matrix must list requiredCapabilities")
    for capability in [
        "structured_layout_authority",
        "workspace_state",
        "shell_chrome_transient_policy",
        "native_launch_visibility",
        "live_capture_evidence",
        "agent_control_affordances",
        "deployment_operations",
    ]:
        if capability not in required:
            failures.append(f"capability matrix missing deployed WM capability: {capability}")

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
    if "smithay-rust-backend-spike" not in backend_ids:
        failures.append("capability matrix must include smithay-rust-backend-spike")

    standard = next(
        (backend for backend in matrix.get("backends", []) if backend.get("id") == "standard-wayland-protocol-probe"),
        None,
    )
    if standard:
        standard_capabilities = standard.get("capabilities", {})
        support = standard_capabilities.get("synchronous_input_deny", {}).get("support")
        if support != "missing":
            failures.append("standard-wayland-protocol-probe must keep synchronous_input_deny marked missing until proven otherwise")
        geometry_support = standard_capabilities.get("geometry_control", {}).get("support")
        if geometry_support != "missing":
            failures.append("standard-wayland-protocol-probe must keep geometry_control marked missing until proven otherwise")
        for capability, entry in standard_capabilities.items():
            evidence = entry.get("evidence", "")
            if not evidence.startswith("probe-observations:"):
                failures.append(f"standard-wayland-protocol-probe.{capability} evidence must cite probe-observations")

    smithay = next(
        (backend for backend in matrix.get("backends", []) if backend.get("id") == "smithay-rust-backend-spike"),
        None,
    )
    if smithay:
        if smithay.get("decision") != "deferred_until_nested_native_client_proof":
            failures.append("Smithay backend decision must remain deferred until nested native client proof")
        smithay_capabilities = smithay.get("capabilities", {})
        for capability in ["per_toplevel_capture", "native_launch_visibility", "live_capture_evidence", "deployment_operations"]:
            if smithay_capabilities.get(capability, {}).get("support") != "missing":
                failures.append(f"Smithay {capability} must stay missing until live runtime evidence exists")
        if smithay_capabilities.get("structured_layout_authority", {}).get("support") != "prototype":
            failures.append("Smithay structured_layout_authority must be prototype-only until runtime proof exists")

    validate_standard_probe(probe, standard, required, failures)

    if failures:
        print("\n".join(failures))
        return 1

    print("compositor capability fixtures: OK")
    return 0


def validate_standard_probe(probe: dict, standard: dict | None, required: list[str], failures: list[str]) -> None:
    if probe.get("schema") != "agora-de.standard-protocol-probe.v1":
        failures.append("standard protocol probe has unexpected schema")
    if probe.get("probeId") != "standard-wayland-protocol-probe":
        failures.append("standard protocol probe has unexpected probeId")

    observations = probe.get("observations", [])
    if not isinstance(observations, list) or not observations:
        failures.append("standard protocol probe must list observations")
        return

    observed: dict[str, dict] = {}
    for observation in observations:
        capability = observation.get("capability")
        if capability in observed:
            failures.append(f"standard protocol probe has duplicate observation for {capability}")
        observed[capability] = observation

        if capability not in required:
            failures.append(f"standard protocol probe has unknown capability: {capability}")
        support = observation.get("support")
        if support not in ALLOWED_SUPPORT:
            failures.append(f"standard protocol probe {capability} has invalid support: {support}")
        if not isinstance(observation.get("finding"), str) or not observation["finding"].strip():
            failures.append(f"standard protocol probe {capability} must include finding")

        protocols = observation.get("protocols")
        if not isinstance(protocols, list):
            failures.append(f"standard protocol probe {capability} protocols must be a list")
            continue
        if support == "standard_protocol" and not protocols:
            failures.append(f"standard protocol probe {capability} must list supporting protocols")
        if support == "missing" and protocols:
            failures.append(f"standard protocol probe {capability} must not list protocols when support is missing")
        for protocol in protocols:
            for field in ("name", "interface", "stage", "source"):
                if not isinstance(protocol.get(field), str) or not protocol[field].strip():
                    failures.append(f"standard protocol probe {capability} protocol must include {field}")

    missing = [capability for capability in required if capability not in observed]
    if missing:
        failures.append(f"standard protocol probe missing observations: {', '.join(missing)}")

    if standard is None:
        return

    standard_capabilities = standard.get("capabilities", {})
    for capability in required:
        matrix_support = standard_capabilities.get(capability, {}).get("support")
        probe_support = observed.get(capability, {}).get("support")
        if probe_support and probe_support != matrix_support:
            failures.append(
                f"standard protocol probe {capability} support {probe_support} does not match matrix {matrix_support}"
            )


if __name__ == "__main__":
    raise SystemExit(main())

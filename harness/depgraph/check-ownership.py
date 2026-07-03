#!/usr/bin/env python3
import json
import pathlib
import re
import sys
import tomllib


def normalized(value: str) -> str:
    return value.replace("_", "-")


def load_toml(path: pathlib.Path) -> dict:
    with path.open("rb") as handle:
        return tomllib.load(handle)


def package_name_for_key(key: str) -> str:
    return f"@agora-de/{key.rsplit('/', 1)[-1]}"


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    ownership = load_toml(root / "governance" / "ownership.toml")
    failures: list[str] = []

    go_packages = ownership.get("go_package", {})
    for package_dir in sorted(path for path in (root / "go" / "internal").rglob("*") if path.is_dir()):
        if not any(package_dir.glob("*.go")):
            continue
        key = package_dir.relative_to(root).as_posix()
        if key not in go_packages:
            failures.append(f"missing Go ownership entry: {key}")

    for key in sorted(go_packages):
        package_dir = root / key
        if not package_dir.exists():
            failures.append(f"Go ownership entry points at missing package: {key}")
        elif not any(package_dir.glob("*.go")):
            failures.append(f"Go ownership entry has no Go files: {key}")

    go_import_re = re.compile(r'"(agora-de\.local/go/internal/[a-zA-Z0-9_./-]+)"')
    for package_dir in sorted(path for path in (root / "go" / "internal").rglob("*") if path.is_dir()):
        if not any(package_dir.glob("*.go")):
            continue
        key = package_dir.relative_to(root).as_posix()
        meta = go_packages.get(key, {})
        allowed = meta.get("may_import", [])
        allowed_set = None if allowed == "unrestricted" else set(allowed)

        imports: set[str] = set()
        for source in package_dir.glob("*.go"):
            for match in go_import_re.finditer(source.read_text()):
                import_path = match.group(1)
                import_key = "go/internal/" + import_path.split("/go/internal/", 1)[1]
                if import_key != key:
                    imports.add(import_key)

        for import_key in sorted(imports):
            if import_key not in go_packages:
                failures.append(f"{key} imports unknown Go internal package {import_key}")
            if allowed_set is not None and import_key not in allowed_set:
                failures.append(f"{key} imports unlisted Go package {import_key}")

    crates = ownership.get("crate", {})
    workspace = load_toml(root / "de-rs" / "Cargo.toml")
    members = workspace.get("workspace", {}).get("members", [])
    internal_crates: dict[str, str] = {}

    for rel in members:
        key = f"de-rs/{rel}"
        cargo_path = root / "de-rs" / rel / "Cargo.toml"
        if key not in crates:
            failures.append(f"missing Rust ownership entry: {key}")
            continue
        if not cargo_path.exists():
            failures.append(f"missing Cargo.toml for workspace member: {key}")
            continue
        cargo = load_toml(cargo_path)
        package_name = cargo.get("package", {}).get("name")
        if package_name:
            internal_crates[package_name] = key

    for rel in members:
        key = f"de-rs/{rel}"
        cargo_path = root / "de-rs" / rel / "Cargo.toml"
        if not cargo_path.exists():
            continue
        cargo = load_toml(cargo_path)
        meta = crates.get(key, {})
        allowed = meta.get("may_depend_on", [])
        forbidden = set(meta.get("may_not_depend_on", []))
        if allowed == "unrestricted":
            allowed_set = None
        else:
            allowed_set = {normalized(item) for item in allowed}

        for section in ("dependencies", "dev-dependencies", "build-dependencies"):
            for dep_name, dep_spec in cargo.get(section, {}).items():
                package_name = dep_spec.get("package", dep_name) if isinstance(dep_spec, dict) else dep_name
                if package_name not in internal_crates:
                    continue
                normalized_dep = normalized(package_name)
                if normalized_dep in {normalized(item) for item in forbidden}:
                    failures.append(f"{key} depends on forbidden crate {package_name}")
                if allowed_set is not None and normalized_dep not in allowed_set:
                    failures.append(f"{key} depends on unlisted crate {package_name}")

    packages = ownership.get("package", {})
    ts_packages_root = root / "ts" / "packages"
    known_package_names = {package_name_for_key(key) for key in packages}
    import_re = re.compile(r"(?:from\s+|import\s+(?:type\s+)?|import\s*\(\s*)[\"'](@agora-de/[a-z0-9-]+)(/[^\"']*)?[\"']")

    for package_dir in sorted(path for path in ts_packages_root.iterdir() if path.is_dir()):
        key = f"ts/packages/{package_dir.name}"
        if key not in packages:
            failures.append(f"missing TypeScript ownership entry: {key}")
            continue
        package_json = json.loads((package_dir / "package.json").read_text())
        package_name = package_json.get("name")
        if package_name not in known_package_names:
            failures.append(f"unexpected package name for {key}: {package_name}")

        meta = packages[key]
        allowed = meta.get("may_import", [])
        allowed_set = None if allowed == "unrestricted" else set(allowed)
        forbidden = set(meta.get("may_not_import", []))

        imports: set[str] = set()
        for source in (package_dir / "src").rglob("*.ts"):
            text = source.read_text()
            for match in import_re.finditer(text):
                dep = match.group(1)
                suffix = match.group(2)
                if suffix:
                    failures.append(f"{key} deep-imports {dep}{suffix}; use root barrel")
                if dep != package_name:
                    imports.add(dep)

        for section in ("dependencies", "devDependencies", "peerDependencies"):
            for dep in package_json.get(section, {}):
                if dep.startswith("@agora-de/") and dep != package_name:
                    imports.add(dep)

        for dep in sorted(imports):
            if dep in forbidden:
                failures.append(f"{key} imports forbidden package {dep}")
            if allowed_set is not None and dep not in allowed_set:
                failures.append(f"{key} imports unlisted package {dep}")

    if failures:
        print("\n".join(failures))
        return 1

    print("ownership depgraph: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

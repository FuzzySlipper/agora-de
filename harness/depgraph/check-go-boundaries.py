#!/usr/bin/env python3
import pathlib
import re
import sys


def main() -> int:
    root = pathlib.Path(sys.argv[1])
    forbidden_path = root / "harness" / "policy" / "forbidden-patterns.txt"
    patterns = [
        re.compile(line.strip())
        for line in forbidden_path.read_text().splitlines()
        if line.strip() and not line.startswith("#")
    ]

    failures: list[str] = []
    for path in sorted((root / "go").rglob("*.go")):
        text = path.read_text()
        rel = path.relative_to(root)
        for pattern in patterns:
            if pattern.search(text):
                failures.append(f"{rel}: forbidden boundary reference: {pattern.pattern}")

    if failures:
        print("\n".join(failures))
        return 1

    print("Go boundaries: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())


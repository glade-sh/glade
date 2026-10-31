#!/usr/bin/env python3
"""Repeat the explicitly listed timing-sensitive Go tests in CI."""
import re
import subprocess
import sys
from pathlib import Path


def main():
    inventory = Path(sys.argv[1]) if len(sys.argv) == 2 else Path(__file__).with_name("ci-repeat-tests.txt")
    rows = []
    for number, line in enumerate(inventory.read_text().splitlines(), 1):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split()
        if len(fields) != 2 or not re.fullmatch(r"\./internal/[a-z0-9_/]+", fields[0]) or not re.fullmatch(r"Test[A-Za-z0-9_]+", fields[1]):
            raise ValueError(f"invalid inventory row {number}")
        row = tuple(fields)
        if row in rows:
            raise ValueError(f"duplicate inventory row {number}")
        rows.append(row)
    if not rows:
        raise ValueError("empty repeat inventory")
    for package, name in rows:
        found = subprocess.run(["go", "test", "-list", "^" + name + "$", package], check=True, text=True, stdout=subprocess.PIPE)
        if name not in found.stdout.splitlines():
            raise ValueError(f"missing test {package} {name}")
        print(f"Repeating {package} {name}", flush=True)
        subprocess.run(["go", "test", "-count=20", "-p=1", "-timeout=0", "-run", "^" + name + "$", package], check=True)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(f"repeat-run failed: {error}", file=sys.stderr)
        sys.exit(1)

#!/usr/bin/env python3
"""Preserve native field sequence from the owned standard describe catalog.

The broad catalog is the authoritative input. The existing packed describe
format deliberately sorts fields, so its order cannot serve FIELDS(STANDARD).
This compact projection contains names only and leaves the source untouched.
"""

import gzip
import json
import re
from pathlib import Path


def main():
    storage = Path(__file__).resolve().parents[1] / "internal" / "storage"
    source = storage / "standard_describe_catalog.json.gz"
    destination = storage / "standard_field_order.tsv.gz"
    with gzip.open(source, "rt", encoding="utf-8") as stream:
        describes = json.load(stream)["describes"]

    rows = []
    for name, describe in sorted(describes.items()):
        fields = [field["name"] for field in describe["fields"]]
        if any(not re.fullmatch(r"[A-Za-z][A-Za-z0-9_]*", part) for part in [name, *fields]):
            raise ValueError(f"invalid object or field name in {name}")
        if len(fields) != len(set(fields)):
            raise ValueError(f"duplicate field in {name}")
        rows.append(name + "\t" + ",".join(fields) + "\n")

    # Empty filename and zero timestamp make the gzip header reproducible.
    with destination.open("wb") as output:
        with gzip.GzipFile(fileobj=output, mode="wb", filename="", mtime=0) as compressed:
            compressed.write("".join(rows).encode("utf-8"))


if __name__ == "__main__":
    main()

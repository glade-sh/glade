#!/usr/bin/env python3
"""Split an already validated shard selection into bounded exact-name regexes."""
import json
import sys

LIMIT = 100000


def chunks(selected):
    names = selected["tests"]
    parts = []
    length = len("^(?:)$")
    for name in names:
        # Planner validation limits names to ASCII letters, digits and underscores.
        size = len(name.encode("utf-8")) + bool(parts)
        if parts and length + size > LIMIT:
            yield "^(?:" + "|".join(parts) + ")$"
            parts = []
            length = len("^(?:)$")
            size = len(name.encode("utf-8"))
        if length + size > LIMIT:
            raise ValueError("test name exceeds shard regex limit")
        parts.append(name)
        length += size
    if parts:
        yield "^(?:" + "|".join(parts) + ")$"


if __name__ == "__main__":
    with open(sys.argv[1], encoding="utf-8") as source:
        for regex in chunks(json.load(source)):
            print(regex)

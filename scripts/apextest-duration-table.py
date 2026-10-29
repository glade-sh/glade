#!/usr/bin/env python3
"""Convert one `go test -json -count=1` log to Apex test shard weights.

Example (run tests on an approved runner):
  python3 scripts/apextest-duration-table.py --source 'SHA; host; Go/env' \
    < events.json > internal/apextest/testdata/shard_durations.json

Top-level run-to-terminal spans include parallel children. A top-level test's
own pause/cont interval is excluded so waiting for other tests is not charged.
Go's reported parent Elapsed can exclude its parallel children, making it an
unsuitable weight. Full-package performance claims still require a complete,
unfiltered run on a quiet host; this tool cannot infer the invocation or load.
"""

import argparse
import datetime
import json
import re
import sys


PACKAGE = "github.com/glade-sh/glade/internal/apextest"
TEST_NAME = re.compile(r"Test[A-Za-z0-9_]*\Z")


def duration_table(events, source, allow_partial=False):
    started = False
    complete = False
    tests = {}
    durations = {}
    for event in events:
        if not isinstance(event, dict) or event.get("Package") != PACKAGE:
            raise ValueError("expected events from the Apex test package only")
        action = event.get("Action")
        name = event.get("Test")
        if complete:
            raise ValueError("event after package completion")
        if not name:
            if action == "start":
                if started:
                    raise ValueError("repeated package start; use -count=1")
                started = True
            elif action in ("pass", "fail", "skip"):
                if not started:
                    raise ValueError("package finished without starting")
                complete = True
            continue
        if not started:
            raise ValueError("test event before package start")
        if "/" in name or action not in ("run", "pause", "cont", "pass", "fail", "skip"):
            continue
        if not TEST_NAME.fullmatch(name):
            raise ValueError("unexpected top-level test name: " + name)
        stamp = datetime.datetime.fromisoformat(event["Time"].replace("Z", "+00:00"))
        if stamp.tzinfo is None:
            raise ValueError("event timestamp needs a timezone")
        if action == "run":
            if name in tests:
                raise ValueError("repeated test: " + name + "; use -count=1")
            tests[name] = {"active": stamp, "last": stamp, "seconds": 0.0}
            continue
        if name not in tests or name in durations:
            raise ValueError("event outside test lifetime: " + name)
        state = tests[name]
        if stamp < state["last"]:
            raise ValueError("timestamp moved backwards: " + name)
        state["last"] = stamp
        if action == "cont":
            if state["active"] is not None:
                raise ValueError("resume without pause: " + name)
            state["active"] = stamp
            continue
        if state["active"] is None:
            raise ValueError("test did not resume: " + name)
        state["seconds"] += (stamp - state["active"]).total_seconds()
        state["active"] = None
        if action != "pause":
            durations[name] = round(state["seconds"] * 1000)
    partial = not complete or len(durations) != len(tests)
    if partial and not allow_partial:
        raise ValueError("incomplete package or unfinished tests; use --allow-partial for scheduling hints only")
    if not durations:
        raise ValueError("no completed top-level tests")
    if partial:
        source += "; PARTIAL: completed tests only, not full-package performance evidence"
    source += "; wall spans include parallel children and exclude top-level pauses"
    return {
        "version": 1,
        "package": PACKAGE,
        "source": source,
        "tests": [{"name": name, "durationMillis": durations[name]} for name in sorted(durations)],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--source", required=True, help="measurement identity: SHA, host and Go/environment settings")
    parser.add_argument("--allow-partial", action="store_true", help="export explicitly labelled scheduling hints from an interrupted log")
    parser.add_argument("--input", action="append", help="completed/focused JSON log; repeat to merge measured families using their maximum span")
    args = parser.parse_args()
    try:
        if args.input:
            merged = {}
            sources = []
            for path in args.input:
                with open(path) as stream:
                    current = duration_table((json.loads(line) for line in stream if line.strip()), args.source, args.allow_partial)
                sources.append(current["source"])
                for row in current["tests"]:
                    merged[row["name"]] = max(merged.get(row["name"], 0), row["durationMillis"])
            table = {"version": 1, "package": PACKAGE,
                     "source": args.source + "; merged maximum spans from separate selections; scheduling hints, not a matched full-package measurement" +
                               ("; PARTIAL inputs included" if any("PARTIAL" in value for value in sources) else ""),
                     "tests": [{"name": name, "durationMillis": merged[name]} for name in sorted(merged)]}
        else:
            events = (json.loads(line) for line in sys.stdin if line.strip())
            table = duration_table(events, args.source, args.allow_partial)
    except (ValueError, KeyError, TypeError) as error:
        parser.exit(2, "apextest-duration-table: " + str(error) + "\n")
    json.dump(table, sys.stdout, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()

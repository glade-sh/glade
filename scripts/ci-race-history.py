#!/usr/bin/env python3
"""Record measured race test durations from one Race workflow run.

The race plan used to scale checked-in normal-mode durations by one
conservative multiplier because no race-instrumented durations existed. The
scheduled run 44515703479 then showed the multiplier's limit: a test missing
from the normal table was planned at the history median and its shard was
killed, and lanes with no history at all (the CLI lane) were balanced by name
hash. Every shard runner already records `go test -json` events per shard, so
a run's artifacts hold real race-instrumented durations for nearly every
test. This module turns one named run's lane artifacts into the checked-in
race history `scripts/ci-race-history.json`, and the plan prefers those
measurements, applying the multiplier only to tests the run never measured.

Durations come from each lane's test2json events:

  measured test      terminal (pass/fail/skip) timestamp minus its run
                     timestamp
  test killed by a timeout
                     at least its elapsed time at the kill: the lane's last
                     recorded event timestamp minus its run timestamp. The
                     true duration is at least this, so the recorded value is
                     a lower bound and the next refresh can only grow it, and
                     the row is marked `"killed": true` so the plan can keep
                     the multiplier's estimate wherever it is larger.

A lane killed by the outer batch deadline can stop flushing events before
the kill, so a killed row from such a lane can still understate the elapsed
time; the plan's headroom and the next refresh cover that drift. Lanes that
were killed before later shards started contribute no rows for the tests that
never ran; those keep the multiplier fallback until a later run measures
them.

Each lane's wall (from its resource.json) is recorded next to the tests: a
shard wall from a lane whose every started test reached a terminal is a
complete observation, and the lane budgets for sequential multi-shard lanes
are derived from those complete walls instead of per-test sums, because
parallel subtests make the sums overstate the wall several times over.

Refreshing the history from a new run:

    run_id=<github-actions-run-id>
    rm -rf /tmp/glade-race-lanes && mkdir -p /tmp/glade-race-lanes
    # One directory per lane artifact; the pattern excludes the test-binary
    # and partition artifacts, which hold no per-test events.
    gh run view "$run_id" -R glade-sh/glade --json artifacts \\
      --jq '.artifacts[].name' |
      grep -vE '^race-(test-binaries|apextest-partition|lwc-history-input|plan)' |
      while read -r artifact; do
        gh run download "$run_id" -R glade-sh/glade \\
          -D "/tmp/glade-race-lanes/$artifact" -n "$artifact"
      done
    python3 scripts/ci-race-history.py "$run_id" /tmp/glade-race-lanes \\
      scripts/ci-race-history.json

Commit the regenerated file; its recorded source run is the provenance. A
test whose events end without a terminal (the run was killed while it was
running) is recorded at its elapsed time at the kill, so the plan derives a
budget that is large enough for every completed measurement and grows on the
next refresh if the test needs more.

Usage:

  ci-race-history.py RUN_ID ARTIFACTS_ROOT OUTPUT_JSON
"""

import datetime
import json
import math
import re
import sys
from pathlib import Path

TEST_NAME = re.compile(r"Test[A-Za-z0-9_]*\Z")
MODULE = "github.com/glade-sh/glade/"
REPO_URL = "https://github.com/glade-sh/glade"


class HistoryError(SystemExit):
    pass


def fail(message):
    raise HistoryError("ci-race-history: " + message)


def parse_timestamp(value):
    # test2json timestamps are RFC 3339 with nanosecond precision.
    return datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))


def lane_events(path):
    """Return (package, durations, killed) parsed from one events.json.

    durations maps a top-level test name to its measured race duration in
    whole milliseconds; killed lists the tests the lane was still running
    when it died, each recorded at its elapsed time at the kill.
    """
    durations = {}
    killed = []
    package = None
    started = {}
    terminals = {}
    last_time = None
    with open(path, encoding="utf-8") as source:
        for number, line in enumerate(source, 1):
            if not line.strip():
                continue
            try:
                event = json.loads(line)
            except ValueError as error:
                fail(f"{path}:{number}: invalid event line: {error}")
            if not isinstance(event, dict):
                fail(f"{path}:{number}: event is not an object")
            if not isinstance(event.get("Package"), str) or not event["Package"].startswith(MODULE):
                fail(f"{path}:{number}: event has an invalid package")
            if package is None:
                package = event["Package"]
            elif package != event["Package"]:
                fail(f"{path}:{number}: lane mixes packages {package} and {event['Package']}")
            try:
                when = parse_timestamp(event["Time"])
            except (KeyError, ValueError):
                fail(f"{path}:{number}: event has no usable timestamp")
            if last_time is None or when > last_time:
                last_time = when
            name, action = event.get("Test"), event.get("Action")
            if not isinstance(name, str) or "/" in name:
                continue
            if not TEST_NAME.fullmatch(name):
                fail(f"{path}:{number}: lane names an invalid test {name!r}")
            if action == "run":
                if name in started:
                    fail(f"{path}:{number}: test {name} starts twice in one lane")
                started[name] = when
            elif action in ("pass", "fail", "skip"):
                terminals.setdefault(name, []).append((action, when))
    if package is None or not started:
        return package, durations, killed
    for name, moments in terminals.items():
        if name not in started:
            continue
        if len(moments) != 1:
            fail(f"{path}: test {name} has {len(moments)} terminals in one lane")
        duration_ms = max(math.ceil((moments[0][1] - started[name]).total_seconds() * 1000), 1)
        if name in durations:
            fail(f"{path}: test {name} was already measured in this lane")
        durations[name] = duration_ms
    for name, when in started.items():
        if name in terminals:
            continue
        duration_ms = max(math.ceil((last_time - when).total_seconds() * 1000), 1)
        killed.append(name)
        if name in durations:
            fail(f"{path}: test {name} was already measured in this lane")
        durations[name] = duration_ms
    return package, durations, killed


def load_lane_wall(lane_dir, killed):
    """Record one lane's wall when its resource evidence survived.

    A wall is complete when every started test reached a terminal; a lane
    killed mid-test (or killed by the batch deadline before the resource
    runner could write) yields no complete wall.
    """
    resource_path = lane_dir / "resource.json"
    try:
        with open(resource_path, encoding="utf-8") as source:
            value = json.load(source)
    except (OSError, ValueError):
        return None
    if not isinstance(value, dict) or set(value) != {
            "schema_version", "lane", "elapsed_seconds", "exit_status",
            "user_seconds", "system_seconds", "max_rss_kb"}:
        fail(f"{resource_path}: resource evidence has an invalid schema")
    elapsed, status = value["elapsed_seconds"], value["exit_status"]
    if (isinstance(elapsed, bool) or not isinstance(elapsed, (int, float)) or
            isinstance(status, bool) or not isinstance(status, int)):
        fail(f"{resource_path}: resource evidence has invalid fields")
    return {"elapsedSeconds": elapsed, "exitStatus": status, "complete": not killed}


def collect(root):
    """Parse every lane events.json (and wall) under one artifacts root."""
    packages = {}
    killed_rows = []
    walls = {}
    lanes = sorted({path.parent for path in root.rglob("events.json")})
    if not lanes:
        fail(f"{root} holds no lane events; download the run's race artifacts first")
    for lane_dir in lanes:
        package, durations, killed = lane_events(lane_dir / "events.json")
        if package is None:
            continue
        rows = packages.setdefault(package, {})
        for name, duration_ms in durations.items():
            incoming = {"durationMillis": duration_ms, "killed": name in killed}
            existing = rows.get(name)
            if existing is None:
                rows[name] = incoming
            elif existing["killed"] and not incoming["killed"]:
                rows[name] = incoming
            elif not existing["killed"] and incoming["killed"]:
                continue
            elif existing["durationMillis"] != incoming["durationMillis"]:
                if not existing["killed"] or not incoming["killed"]:
                    fail(f"{package} test {name} was measured twice with different durations")
                rows[name] = {"durationMillis": max(existing["durationMillis"], incoming["durationMillis"]), "killed": True}
        killed_rows.extend(f"{package}:{name}" for name in killed)
        wall = load_lane_wall(lane_dir, killed)
        if wall is not None:
            walls.setdefault(package, []).append(wall)
    if not packages:
        fail(f"{root} holds no usable lane events")
    return packages, walls, sorted(killed_rows)


def main():
    arguments = sys.argv[1:]
    if len(arguments) != 3 or not arguments[0].isdigit() or arguments[2] == "":
        fail("usage: ci-race-history.py RUN_ID ARTIFACTS_ROOT OUTPUT_JSON")
    run_id, root_text, output_text = arguments
    root = Path(root_text)
    if not root.is_dir():
        fail(f"artifacts root {root} is not a directory")
    packages, walls, killed = collect(root)
    document = {
        "schema_version": 1,
        "source_run_id": int(run_id),
        "source_run_url": f"{REPO_URL}/actions/runs/{run_id}",
        "packages": [
            {
                "package": package,
                "tests": [{"name": name, "durationMillis": row["durationMillis"],
                           "killed": row["killed"]}
                          for name, row in sorted(rows.items())],
                "shardWalls": walls.get(package, []),
            }
            for package, rows in sorted(packages.items())
        ],
    }
    output = Path(output_text)
    with open(output, "w", encoding="utf-8") as target:
        json.dump(document, target, sort_keys=True, indent=2)
        target.write("\n")
    measured = sum(len(rows) for rows in packages.values())
    print(f"ci-race-history: recorded {measured} measured race durations "
          f"from run {run_id} across {len(packages)} packages")
    if killed:
        print("ci-race-history: killed tests recorded at their elapsed time at the kill:")
        for name in killed:
            print(f"  {name}")


if __name__ == "__main__":
    main()

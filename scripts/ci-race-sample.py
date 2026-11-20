#!/usr/bin/env python3
"""Print a `go test -run` regex for the nightly race conformance sample.

The nightly race workflow keeps parallel conformance subtests under the
detector by running a fixed 1-in-25 sample of a package's tests: every test
whose sha1(name) % 25 == 0 and whose normal-build duration is under 120 s,
plus (with --parallel) the package's top-level tests that call t.Parallel().
Nothing is checked in: names come from `go test -list` on stdin, and
durations come from the package's existing shard_durations.json, where a
missing row means a new or fast test and stays eligible.

The ten slowest race tests ever measured (scheduled race run 44541836806,
2026-11-11 and 2026-11-19) are excluded: each cost the nightly run a
double-digit share of its wall time, and the weekly full backstop still
runs them whole.
"""

import argparse
import hashlib
import json
import re
import sys

TEST_HEADER = re.compile(r"^func (Test\w+)\(t \*testing\.T\) \{")
NEXT_FUNC = re.compile(r"^func ")
SAMPLE_MODULUS = 25
SLOWEST_RACE_TESTS = {
    "TestNamesOnlyGlobalDescribeMatchesCompletedSpread",
    "TestCartExtensionDirectProjectNamedIsTestTriggerNativeRoutes",
    "TestMapKeyTypeCannotContainCollectionAliasKind",
    "TestProcessBuilderReviewAdditionalHosts",
    "TestStrictHeapRoutesMatchFullWalk",
    "TestVisualforceRenderMarkupNativeRows",
    "TestL11SalesforceConformanceAPI67",
    "TestL11SalesforceConformanceAPI68",
    "TestL11SalesforceConformanceAPI59",
    "TestC14WarmProjectRoutes",
}
MAX_SAMPLED_NORMAL_MILLIS = 120 * 1000


def duration_table(path):
    """Return {name: durationMillis} from a shard_durations.json table."""
    with open(path, encoding="utf-8") as source:
        rows = json.load(source).get("tests", [])
    if not isinstance(rows, list):
        raise SystemExit(f"race sample: {path} has no test duration rows")
    table = {}
    for row in rows:
        name, millis = row.get("name"), row.get("durationMillis")
        if isinstance(name, str) and isinstance(millis, (int, float)):
            table[name] = millis
    return table


def parallel_top_level_tests(package_dir):
    """Return top-level Test names whose own body calls t.Parallel().

    The body runs from the func header to the next top-level func, which is
    how a reader attributes the call; helpers between tests are never
    attributed to the test above them.
    """
    import glob
    import os

    parallel = set()
    for path in sorted(glob.glob(os.path.join(package_dir, "*_test.go"))):
        current = None
        with open(path, encoding="utf-8") as source:
            for line in source:
                header = TEST_HEADER.match(line)
                if header:
                    current = header.group(1)
                elif NEXT_FUNC.match(line):
                    current = None
                    continue
                if current and "t.Parallel()" in line:
                    parallel.add(current)
    return parallel


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("durations", help="package shard_durations.json")
    parser.add_argument("package_dir", help="package directory with *_test.go")
    parser.add_argument("--parallel", action="store_true",
                        help="also select top-level tests that call t.Parallel()")
    args = parser.parse_args()

    # go test -list also prints a trailing "ok <package> <time>" result line
    # for each package; keep only test names.
    names = [line.strip() for line in sys.stdin if re.match(r"^Test\w+$", line.strip())]
    if not names:
        raise SystemExit("race sample: no test names on stdin")
    if len(names) != len(set(names)):
        raise SystemExit("race sample: duplicate test names on stdin")
    durations = duration_table(args.durations)

    selected = []
    for name in names:
        if name in SLOWEST_RACE_TESTS:
            continue
        sampled = int(hashlib.sha1(name.encode()).hexdigest(), 16) % SAMPLE_MODULUS == 0
        normal_millis = durations.get(name)
        under_cut = normal_millis is None or normal_millis < MAX_SAMPLED_NORMAL_MILLIS
        if sampled and under_cut:
            selected.append(name)
    if args.parallel:
        parallel = parallel_top_level_tests(args.package_dir)
        for name in names:
            if name in SLOWEST_RACE_TESTS:
                continue
            if name in parallel and name not in selected:
                selected.append(name)
    if not selected:
        raise SystemExit("race sample: nothing selected; the sample rule or its inputs are wrong")
    print("^(%s)$" % "|".join(sorted(selected)))


if __name__ == "__main__":
    main()

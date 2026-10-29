#!/usr/bin/env python3
"""Compare complete Go JSON outcome multisets, including native count output.

Repeated -count runs are rejected; use -count=1. Package pass/fail and every
started test must be terminal. Only explicitly named new top-level guard tests
may be added. This validates the supplied selection, not full-suite coverage:
use --registry with the exact binary's -list output for a full-suite claim.
Each test has one partition owner independent of its terminal action. Subtests
must execute inside a started, unfinished top-level family; child-only logs
cannot prove that a registry family ran.
Go framework progress and durations are omitted. Test payloads retain their
text and multiplicity; only test source line numbers and supplied --root paths
are normalized. Add timing-only log patterns deliberately, never drop errors.
"""
import argparse
from collections import Counter
import json
import re
import sys

TERMINAL = {"pass", "fail", "skip"}


def evidence(events, roots=()):
    package = None
    complete = False
    started = set()
    finished = set()
    active_children = Counter()
    outcomes = Counter()
    payload = Counter()
    output = {}
    for event in events:
        if complete:
            raise ValueError("events after package completion")
        current = event.get("Package")
        if not current:
            raise ValueError("event without package")
        action, name = event.get("Action"), event.get("Test")
        if package is None:
            if action != "start" or name:
                raise ValueError("log must begin with package start")
            package = current
        elif package != current or action == "start":
            raise ValueError("mixed or repeated package")
        if not name and action in TERMINAL:
            if started != finished:
                raise ValueError("package ended with unfinished tests")
            if action == "pass" and any(key[2] == "fail" for key in outcomes):
                raise ValueError("passing package contains failing test")
            if action == "fail" and not any(key[2] == "fail" for key in outcomes):
                raise ValueError("package failed without a terminal test failure (interruption/build failure)")
            complete = True
        if not name:
            continue
        if action == "run":
            if name in started:
                raise ValueError("repeated test; require -count=1: " + name)
            # t.Run labels can contain slashes, so only the top-level family
            # is unambiguously a parent in the JSON name. Intermediate path
            # components may be labels, even among concurrent siblings.
            parent, separator, _ = name.partition('/')
            if separator:
                if parent not in started or parent in finished:
                    raise ValueError("child outside parent lifetime: " + name)
                active_children[parent] += 1
            started.add(name)
        elif action in TERMINAL:
            if name not in started or name in finished:
                raise ValueError("missing start or repeated terminal: " + name)
            if active_children[name]:
                raise ValueError("parent ended with unfinished children: " + name)
            finished.add(name)
            parent, separator, _ = name.partition('/')
            if separator:
                active_children[parent] -= 1
            outcomes[(package, name, action)] += 1
        elif action == "output":
            if name not in started:
                raise ValueError("output without test execution: " + name)
            # test2json splits long lines at arbitrary event boundaries. Keep
            # each test's stream separate even when parallel siblings interleave.
            output.setdefault(name, []).append(event.get("Output", ""))
    if not complete:
        raise ValueError("incomplete package")
    for name, chunks in output.items():
        for line in "".join(chunks).splitlines():
            if re.match(r"^(?:=== |\s*--- (?:PASS|FAIL|SKIP):|PASS$|FAIL$)", line):
                continue
            text = re.sub(r"^\s*(?:[^\s:]+\.go):\d+:\s*", "", line)
            for root in roots:
                text = text.replace(root, "<root>")
            if text.strip():
                payload[(package, name, text)] += 1
    return outcomes, payload


def compare(before, after, additions, roots=(), registry=None):
    def aggregate(logs):
        outcomes, payload = Counter(), Counter()
        owners = set()
        for log in logs:
            current, text = evidence(log, roots)
            identities = {(package, name) for package, name, _ in current}
            if owners & identities:
                raise ValueError("duplicate executed outcomes across partitions")
            owners.update(identities)
            outcomes.update(current)
            payload.update(text)
        if not outcomes:
            raise ValueError("empty aggregate test execution")
        return outcomes, payload
    left, left_text = aggregate(before)
    right, right_text = aggregate(after)
    original = {name.split('/')[0] for _, name, _ in left}
    if registry is not None and original != set(registry):
        raise ValueError("baseline does not exactly cover the binary registry")
    allowed = set(additions)
    if allowed & original:
        raise ValueError("expected additions overlap existing tests")
    actual = {name.split('/')[0] for _, name, _ in right} - original
    if actual != allowed:
        raise ValueError("new guard set differs from explicitly expected additions")
    if any(action != "pass" for (_, name, action) in right if name.split('/')[0] in allowed):
        raise ValueError("added guard failed or skipped")
    def existing(counter):
        return Counter({key: n for key, n in counter.items() if key[1].split('/')[0] not in allowed})
    if left != existing(right):
        raise ValueError("terminal outcome multiset differs")
    if left_text != existing(right_text):
        missing = list((left_text - existing(right_text)).items())[:3]
        added = list((existing(right_text) - left_text).items())[:3]
        raise ValueError("test payload differs: missing=" + repr(missing) + " added=" + repr(added))
    return {"equivalent": True, "existingOutcomes": sum(left.values()),
            "addedGuards": sorted(allowed), "payloadLines": sum(left_text.values()),
            "registryChecked": registry is not None}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--before', nargs='+', required=True)
    parser.add_argument('--after', nargs='+', required=True)
    parser.add_argument('--added', action='append', default=[])
    parser.add_argument('--root', action='append', default=[])
    parser.add_argument('--registry')
    args = parser.parse_args()
    def logs(paths):
        for path in paths:
            with open(path) as source:
                yield (json.loads(line) for line in source if line.strip())
    registry = None
    if args.registry:
        with open(args.registry) as source:
            registry = [line.strip() for line in source if re.fullmatch(r'Test\w+\n?', line)]
        if not registry or len(registry) != len(set(registry)):
            parser.error('empty or duplicated registry')
    try:
        result = compare(logs(args.before), logs(args.after), args.added, args.root, registry)
    except (ValueError, KeyError, TypeError) as error:
        parser.exit(2, 'apextest-outcomes: ' + str(error) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()

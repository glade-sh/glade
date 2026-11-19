#!/usr/bin/env python3
"""Assign the PR lane's remaining Go packages to pool workers by duration.

Reads the selected-package JSON list on stdin and prints the packages assigned
to one worker, one import path per line. Every PR pool worker runs this script
with the same selection and the same CI package manifest, so every worker
derives the same deterministic partition without a central planner: packages
owned by a dedicated worker (prDedicatedPackages) are skipped, the rest are
placed longest-measured-first onto the least-loaded worker, ties broken by the
lowest worker index and then by import path. A package without a measured
entry uses the manifest's conservative default estimate, so new packages still
run and never stack onto one worker. Invalid input fails closed instead of
producing an empty or partial assignment.
"""
import argparse
import json
import os
import sys

MANIFEST_DEFAULT = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'ci-package-lanes.json')


def fail(message):
    print(f'ci-pr-assign: {message}', file=sys.stderr)
    sys.exit(2)


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument('--manifest', default=MANIFEST_DEFAULT,
                        help='CI package manifest (default: scripts/ci-package-lanes.json)')
    parser.add_argument('--worker', type=int, required=True, help='zero-based pool worker index')
    parser.add_argument('--workers', type=int, required=True, help='number of pool workers')
    args = parser.parse_args()
    if args.workers < 1:
        fail('--workers must be positive')
    if not 0 <= args.worker < args.workers:
        fail(f'--worker must be between 0 and {args.workers - 1}')
    try:
        with open(args.manifest, encoding='utf-8') as source:
            manifest = json.load(source)
    except (OSError, ValueError) as error:
        fail(f'manifest unreadable: {error}')
    for field in ('prDedicatedPackages', 'prWorkerSeconds', 'prWorkerSecondsDefault'):
        if field not in manifest:
            fail(f'manifest missing {field}')
    dedicated = manifest['prDedicatedPackages']
    seconds = manifest['prWorkerSeconds']
    default = manifest['prWorkerSecondsDefault']
    if not isinstance(dedicated, list) or not all(isinstance(pkg, str) and pkg for pkg in dedicated):
        fail('prDedicatedPackages must be a list of import paths')
    if not isinstance(seconds, dict):
        fail('prWorkerSeconds must be an object')
    if not isinstance(default, int) or isinstance(default, bool) or default < 1:
        fail('prWorkerSecondsDefault must be a positive integer')
    for pkg, estimate in seconds.items():
        if not isinstance(pkg, str) or not isinstance(estimate, int) or isinstance(estimate, bool) or estimate < 1:
            fail(f'prWorkerSeconds entry for {pkg!r} must be a positive integer')
        if pkg in dedicated:
            fail(f'{pkg} is dedicated to another worker and cannot carry a pool estimate')
    try:
        selected = json.load(sys.stdin)
    except ValueError as error:
        fail(f'selection unreadable: {error}')
    if not isinstance(selected, list) or not all(isinstance(pkg, str) and pkg for pkg in selected):
        fail('selection must be a JSON list of import paths')
    if len(selected) != len(set(selected)):
        fail('selection contains duplicate packages')
    pool = [pkg for pkg in selected if pkg not in dedicated]
    order = sorted(pool, key=lambda pkg: (-seconds.get(pkg, default), pkg))
    loads = [0] * args.workers
    buckets = [[] for _ in range(args.workers)]
    for pkg in order:
        worker = min(range(args.workers), key=lambda index: (loads[index], index))
        loads[worker] += seconds.get(pkg, default)
        buckets[worker].append(pkg)
    for pkg in sorted(buckets[args.worker]):
        print(pkg)


if __name__ == '__main__':
    main()

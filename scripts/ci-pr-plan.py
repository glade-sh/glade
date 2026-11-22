#!/usr/bin/env python3
"""Build the PR worker matrix from a selected package list on stdin."""
import argparse
import json
import os
import re
import subprocess
import sys

ROOT = 'github.com/glade-sh/glade/'
BROWSER = re.compile(r'/internal/(server|playground|visualforce|lwc|lwcbrowser|lwcruntime)(/|$)')
SERVICE_PACKAGES = tuple(ROOT + 'internal/' + p for p in ('server', 'playground'))
GUARD_PACKAGES = {ROOT + 'internal/repoguard', ROOT + 'scripts'}
CLI_PACKAGE = ROOT + 'internal/gladecli'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--manifest', default=os.path.join(os.path.dirname(__file__), 'ci-package-lanes.json'))
    args = parser.parse_args()
    selected = json.load(sys.stdin)
    if not isinstance(selected, list) or not all(isinstance(p, str) and p for p in selected) or len(selected) != len(set(selected)):
        raise ValueError('selection must be a JSON list of distinct package paths')
    # The workflow route loop is the existing source of the shard topology.
    workflow = os.path.join(os.path.dirname(__file__), '..', '.github', 'workflows', 'ci.yml')
    with open(workflow, encoding='utf-8') as source:
        workflow_text = source.read()
    match = re.search(r'^          for spec in ([a-z0-9/: -]+); do$', workflow_text, re.MULTILINE)
    if not match:
        raise ValueError('PR shard route loop missing from workflow')
    routes = []
    for spec in match.group(1).split():
        route, package, base, count = spec.split(':')
        base, count = int(base), int(count)
        if not route or not package or base < 0 or count < 1 or base + count > 44:
            raise ValueError(f'invalid PR route: {spec}')
        routes.append((package, base, count))
    with open(args.manifest, encoding='utf-8') as source:
        manifest = json.load(source)
    dedicated = set(manifest['prDedicatedPackages'])
    non_sharded = set(SERVICE_PACKAGES) | GUARD_PACKAGES
    routed = {ROOT + 'internal/' + package for package, _, _ in routes}
    if routed != dedicated - non_sharded:
        raise ValueError('PR shard routes do not match dedicated packages')
    occupied = [i for _, base, count in routes for i in range(base, base + count)]
    if len(occupied) != len(set(occupied)):
        raise ValueError('PR shard routes overlap')
    workers = {i: {'shard': i, 'packages': [], 'pool': []} for i in range(45)}
    for package, base, count in routes:
        pkg = ROOT + 'internal/' + package
        if pkg in selected:
            for shard in range(base, base + count):
                workers[shard]['packages'].append(pkg)
    cli = CLI_PACKAGE
    if cli in selected:
        workers[22]['packages'].append(cli)
    for pkg in SERVICE_PACKAGES:
        if pkg in selected:
            workers[44]['packages'].append(pkg)
    assign = os.path.join(os.path.dirname(__file__), 'ci-pr-assign.py')
    for index in range(12):
        result = subprocess.run([sys.executable, assign, '--manifest', args.manifest,
                                 '--worker', str(index), '--workers', '12'],
                                input=json.dumps(selected), text=True, capture_output=True, check=True)
        pool = result.stdout.splitlines()
        workers[index + 12]['pool'] = pool
        workers[index + 12]['packages'].extend(pool)
    include = []
    for worker in workers.values():
        if worker['packages']:
            worker['browser'] = worker['shard'] >= 12 and any(BROWSER.search(p) for p in worker['packages'])
            include.append(worker)
    if not include:
        include = [{'shard': -1, 'packages': [], 'pool': [], 'browser': False}]
    print(json.dumps({'include': include}, separators=(',', ':')))


if __name__ == '__main__':
    main()

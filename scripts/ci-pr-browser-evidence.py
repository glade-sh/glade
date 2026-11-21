#!/usr/bin/env python3
"""Merge PR worker browser evidence and run the Browser workflow validator."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from ci_pr_browser_authorities import expected_tests, validator_source

source, target = map(Path, sys.argv[1:])
validator = validator_source()
expected = expected_tests()
latest = {}
for directory in source.iterdir():
    match = re.fullmatch(r'pr-browser-\d+-(\d+)-(-?\d+)', directory.name)
    if directory.is_dir() and match:
        attempt, shard = map(int, match.groups())
        if shard not in latest or attempt > latest[shard][0]:
            latest[shard] = (attempt, directory)
roots = [value[1] for value in latest.values()] if latest else [source]
events = []
for path in sorted(path for root in roots for path in root.rglob('events.jsonl')):
    for line in path.read_text().splitlines():
        event = json.loads(line)
        key = (event.get('Package'), event.get('Test', '').split('/', 1)[0])
        if key in expected:
            events.append(line)
node_logs = [path for root in roots for path in root.rglob('node-test.log')]
if len(node_logs) != 1:
    raise SystemExit(f'expected one Node test log, found {len(node_logs)}')
artifacts = target / 'ci-artifacts/browser'
artifacts.mkdir(parents=True, exist_ok=True)
(artifacts / 'go-test.json').write_text('\n'.join(events) + '\n')
(artifacts / 'node-test.log').write_bytes(node_logs[0].read_bytes())
env = dict(os.environ, GITHUB_WORKSPACE=str(target.resolve()))
subprocess.run([sys.executable, '-c', validator], env=env, check=True)

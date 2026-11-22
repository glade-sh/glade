#!/usr/bin/env python3
"""Validate every planned PR worker's evidence, including cross-shard unions."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

plan_path, input_path, output_path = map(Path, sys.argv[1:])
root = Path(__file__).resolve().parent.parent
workflow = (root / '.github/workflows/ci.yml').read_text()
match = re.search(r'^          for spec in ([a-z0-9/: -]+); do$', workflow, re.MULTILINE)
if not match:
    raise ValueError('PR shard routes missing')
routes = {}
for item in match.group(1).split():
    route, package, base, count = item.split(':')
    routes['github.com/glade-sh/glade/internal/' + package] = (route, int(base), int(count))
plan = json.loads(plan_path.read_text())
workers = plan['include']
expected = {w['shard']: w for w in workers}
if len(expected) != len(workers) or not expected:
    raise ValueError('invalid worker plan')
latest = {}
for directory in input_path.iterdir():
    match = re.fullmatch(r'pr-authority-\d+-(\d+)-(-?\d+)', directory.name)
    if not directory.is_dir() or not match:
        raise ValueError('unexpected authority artifact ' + directory.name)
    attempt, shard = map(int, match.groups())
    if shard not in expected:
        raise ValueError(f'unplanned worker {shard}')
    if shard not in latest or attempt > latest[shard][0]:
        latest[shard] = (attempt, directory)
if set(latest) != set(expected):
    raise ValueError('missing planned worker evidence: ' + str(sorted(set(expected) - set(latest))))
output_path.mkdir(parents=True, exist_ok=True)
deferred = []
no_tests = []
for index, worker in expected.items():
    directory = latest[index][1]
    if json.loads((directory / 'identity.json').read_text()) != {'shard': index}:
        raise ValueError(f'worker {index} identity mismatch')
    for name, packages in [('pool', worker['pool']),
                           ('service', [p for p in worker['packages'] if p not in worker['pool'] and p not in routes])]:
        if packages:
            evidence = directory / name
            subprocess.run([sys.executable, str(root / 'scripts/ci-pr-package-discovery.py'),
                            'validate', str(evidence), *packages], cwd=root, check=True)
            subprocess.run(['bash', str(root / 'scripts/ci-go-test.sh'), 'package-summary',
                            'remaining-go', f'PR {name} {index}', str(evidence / 'events.json'),
                            str(output_path / f'{name}-{index}.json'), str(evidence / 'metadata.tsv'), *packages],
                           cwd=root, check=True)
            deferred.extend(json.loads((output_path / f'{name}-{index}.json').read_text())['deferred'])
            no_tests.extend(line.split('\t', 1)[0] for line in (evidence / 'metadata.tsv').read_text().splitlines() if line.endswith('\tno-tests'))
for package, (route, base, count) in routes.items():
    assigned = [w['shard'] for w in workers if package in w['packages']]
    if not assigned:
        continue
    if not set(range(base, base + count)).issubset(assigned):
        raise ValueError(f'incomplete shard assignment for {package}')
    directories = [str(latest[index][1] / 'shard') for index in range(base, base + count)]
    command = ['bash', '-c', 'source "$1"; shift; refresh_test_history "$@"', 'union',
               str(root / 'scripts/ci-go-test.sh'), package, route, route, '0', str(count),
               str(output_path / (route + '-history.json')), *directories]
    subprocess.run(command, cwd=root, check=True)
    for index in range(base, base + count):
        shard = latest[index][1] / 'shard'
        metadata = output_path / f'{route}-{index}-metadata.tsv'
        metadata.write_text(package + '\thas-tests\n')
        summary = output_path / f'{route}-{index}-package.json'
        env = dict(os.environ, CI_SHARD_SELECTION_FOR_SUMMARY=str(shard / 'selected-shard.json'))
        subprocess.run(['bash', str(root / 'scripts/ci-go-test.sh'), 'package-summary',
                        'gladecli-ci' if route == 'gladecli' else 'remaining-go',
                        f'PR {route} {index}', str(shard / 'events.json'), str(summary), str(metadata), package],
                       cwd=root, check=True, env=env)
        deferred.extend(json.loads(summary.read_text())['deferred'])
    extra = sorted(set(assigned) - set(range(base, base + count)))
    if extra:
        # The only non-shard route is the CLI's selected doctor tests. Derive
        # their exact selection from the executor rather than maintaining a list.
        m = re.search(r"go test (?:-json )?-count=1 -timeout=0 -run '\^\(([^']+)\)\$' ./internal/gladecli", workflow)
        if package != 'github.com/glade-sh/glade/internal/gladecli' or not m or len(extra) != 1:
            raise ValueError('unexpected non-shard dedicated authority')
        tests = set(m.group(1).split('|'))
        doctor = latest[extra[0]][1] / 'doctor'
        subprocess.run(['bash', str(root / 'scripts/ci-go-test.sh'), 'package-summary',
                        'gladecli-ci', 'PR doctor', str(doctor / 'events.json'),
                        str(output_path / 'doctor-summary.json'), str(doctor / 'metadata.tsv'), package],
                       cwd=root, check=True)
        deferred.extend(json.loads((output_path / 'doctor-summary.json').read_text())['deferred'])
        events = [json.loads(line) for line in (doctor / 'events.json').read_text().splitlines()]
        terminals = [e for e in events if e.get('Package') == package and isinstance(e.get('Test'), str) and '/' not in e['Test'] and e.get('Action') in ('pass', 'fail', 'skip')]
        if len(terminals) != len(tests) or {e['Test'] for e in terminals} != tests or any(e['Action'] != 'pass' for e in terminals):
            raise ValueError('doctor terminal results do not match selection')
(output_path / 'summary.json').write_text(json.dumps({'valid': True, 'workers': sorted(expected), 'deferred': deferred, 'noTests': sorted(no_tests)}, sort_keys=True) + '\n')

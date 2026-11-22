#!/usr/bin/env python3
"""Record and check the exact top-level test union of assigned packages."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

mode, directory, *packages = sys.argv[1:]
directory = Path(directory)
if not packages or len(packages) != len(set(packages)):
    raise ValueError('invalid assigned packages')
resolved = []
for package in packages:
    if not (package.startswith('.') or os.path.isabs(package)):
        resolved.append(package)
        continue
    result = subprocess.run(['go', 'list', '-f', '{{.ImportPath}}{{"\t"}}{{if or .TestGoFiles .XTestGoFiles}}has-tests{{else}}no-tests{{end}}', package],
                            text=True, capture_output=True, check=True)
    rows = result.stdout.splitlines()
    fields = rows[0].split('\t') if len(rows) == 1 else []
    if len(fields) != 2 or not fields[0] or fields[1] not in ('has-tests', 'no-tests'):
        raise ValueError('invalid assigned package: ' + package)
    resolved.append(fields[0])
if len(resolved) != len(set(resolved)):
    raise ValueError('duplicate assigned package identities')
packages = resolved
path = directory / 'discovery.json'
if mode == 'collect':
    discovery = {}
    selection = os.environ.get('CI_TEST_RUN_REGEX', '^Test')
    for package in packages:
        result = subprocess.run(['go', 'test', '-vet=off', '-list', selection, package],
                                text=True, capture_output=True, check=True)
        lines = result.stdout.splitlines()
        if not lines or not (re.fullmatch(r'ok\s+' + re.escape(package) + r'(?:\s+.*)?', lines[-1])
                             or re.fullmatch(r'\?\s+' + re.escape(package) + r'\s+\[no test files\]', lines[-1])):
            raise ValueError('invalid package discovery trailer: ' + package)
        names = lines[:-1]
        if 'CI_TEST_RUN_REGEX' in os.environ and not names:
            raise ValueError('selected authority discovery is empty: ' + package)
        if any(not re.fullmatch(r'Test[A-Za-z0-9_]*', name) for name in names) or len(names) != len(set(names)):
            raise ValueError('invalid or duplicate test discovery: ' + package)
        discovery[package] = sorted(names)
    path.write_text(json.dumps(discovery, sort_keys=True) + '\n')
elif mode == 'validate':
    discovery = json.loads(path.read_text())
    if set(discovery) != set(packages):
        raise ValueError('discovery packages differ from assignment')
    terminals = {package: {} for package in packages}
    for line in (directory / 'events.json').read_text().splitlines():
        event = json.loads(line)
        package, name, action = event.get('Package'), event.get('Test'), event.get('Action')
        if package in terminals and isinstance(name, str) and re.fullmatch(r'Test[A-Za-z0-9_]*', name) and action in ('pass', 'fail', 'skip'):
            terminals[package].setdefault(name, []).append(action)
    for package, names in discovery.items():
        if not isinstance(names, list) or names != sorted(set(names)):
            raise ValueError('invalid discovery for ' + package)
        if set(terminals[package]) != set(names) or any(len(actions) != 1 for actions in terminals[package].values()):
            raise ValueError('terminal test union differs from discovery for ' + package)
        # Exact permitted skip reasons are checked by package-summary.
else:
    raise ValueError('invalid discovery mode')

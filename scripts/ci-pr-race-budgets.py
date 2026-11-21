#!/usr/bin/env python3
"""Build the race matrix from measured test and setup budgets."""
import json
import math
from pathlib import Path
import sys

config = json.loads(Path(sys.argv[1]).read_text())
selected = json.load(sys.stdin)
if not isinstance(selected, list) or not all(isinstance(p, str) and p for p in selected) or len(set(selected)) != len(selected):
    raise ValueError('race selection must contain distinct package names')
default = config['default']
def valid_minutes(test, job):
    return type(test) is int and type(job) is int and 0 < test < job <= 360
if not valid_minutes(default['testMinutes'], default['jobMinutes']):
    raise ValueError('invalid default budget')
measured = {}
for sample in config['samples']:
    package = sample['package']
    values = [sample[k] for k in ('testSeconds', 'multiplier', 'setupSeconds', 'marginSeconds')]
    if not isinstance(package, str) or not package or package in measured or not all(type(v) in (int, float) and math.isfinite(v) and v >= 0 for v in values) or values[0] <= 0 or values[1] < 1:
        raise ValueError('invalid measured budget')
    test = max(default['testMinutes'], math.ceil(values[0] * values[1] / 60))
    job = max(default['jobMinutes'], test + math.ceil((values[2] + values[3]) / 60))
    if not valid_minutes(test, job):
        raise ValueError('measured budget outside workflow limits')
    measured[package] = (test, job)
if not selected:
    print('[]')
else:
    include = []
    for package in selected:
        test, job = measured.get(package, (default['testMinutes'], default['jobMinutes']))
        include.append({'package': package, 'test_minutes': test, 'job_minutes': job})
    print(json.dumps({'include': include}, separators=(',', ':')))

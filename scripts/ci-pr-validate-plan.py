#!/usr/bin/env python3
"""Reject a PR matrix that omits or misroutes selected packages."""
import importlib.util
import json
import os
import re
import sys

here = os.path.dirname(__file__)
spec = importlib.util.spec_from_file_location('pr_plan', os.path.join(here, 'ci-pr-plan.py'))
plan_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plan_module)
selected = json.loads(sys.argv[1])
plan = json.loads(sys.argv[2])
if not isinstance(selected, list) or len(set(selected)) != len(selected):
    raise ValueError('invalid selected package list')
with open(os.path.join(here, 'ci-package-lanes.json'), encoding='utf-8') as source:
    dedicated = set(json.load(source)['prDedicatedPackages'])
with open(os.path.join(here, '..', '.github', 'workflows', 'ci.yml'), encoding='utf-8') as source:
    workflow = source.read()
match = re.search(r'^          for spec in ([a-z0-9/: -]+); do$', workflow, re.MULTILINE)
if not match:
    raise ValueError('PR shard route loop missing')
routes = {}
for route in match.group(1).split():
    _, package, base, count = route.split(':')
    routes[plan_module.ROOT + 'internal/' + package] = set(range(int(base), int(base) + int(count)))
if set(routes) != dedicated - set(plan_module.SERVICE_PACKAGES) - plan_module.GUARD_PACKAGES:
    raise ValueError('PR routes do not match dedicated packages')
workers = plan['include']
seen = {p: [] for p in selected}
for worker in workers:
    shard = worker['shard']
    packages = worker['packages']
    pool = worker['pool']
    if len(packages) != len(set(packages)) or len(pool) != len(set(pool)):
        raise ValueError('duplicate package')
    for package in packages:
        if package not in seen:
            raise ValueError('unselected package in plan')
        seen[package].append(shard)
        if package in pool:
            if package in dedicated or not 12 <= shard < 24:
                raise ValueError('misrouted pool package')
        elif package in routes:
            if shard not in routes[package] and not (package == plan_module.CLI_PACKAGE and shard == 22):
                raise ValueError('misrouted dedicated package')
        elif package in plan_module.SERVICE_PACKAGES:
            if shard != 44:
                raise ValueError('misrouted service package')
        else:
            raise ValueError('package does not run on this worker')
    if not set(pool).issubset(packages):
        raise ValueError('pool package missing from worker')
for package, shards in seen.items():
    if package in plan_module.GUARD_PACKAGES:
        if shards:
            raise ValueError('guard package assigned to matrix')
    elif package in routes:
        expected = routes[package] | ({22} if package == plan_module.CLI_PACKAGE else set())
        if set(shards) != expected or len(shards) != len(expected):
            raise ValueError('dedicated route is incomplete')
    elif package in plan_module.SERVICE_PACKAGES:
        if shards != [44]:
            raise ValueError('service route is incomplete')
    elif len(shards) != 1:
        raise ValueError('pool route is incomplete')

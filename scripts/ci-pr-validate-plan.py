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
workflow_path = sys.argv[3] if len(sys.argv) > 3 else os.path.join(here, '..', '.github', 'workflows', 'ci.yml')
with open(workflow_path, encoding='utf-8') as source:
    workflow = source.read()
match = re.search(r'^          for spec in ([a-z0-9/: -]+); do$', workflow, re.MULTILINE)
if not match:
    raise ValueError('PR shard route loop missing')
routes = {}
for route in match.group(1).split():
    _, package, base, count = route.split(':')
    routes[plan_module.ROOT + 'internal/' + package] = set(range(int(base), int(base) + int(count)))
# Read special routes from the executor as well as the sharded loop. A plan
# must fail closed if those workflow shapes change or become ambiguous.
lines = workflow.splitlines()
service_routes, cli_routes, pool_routes = [], [], []
for index, line in enumerate(lines):
    following = lines[index + 1] if index + 1 < len(lines) else ''
    service = re.search(r'if \(\( SHARD == (\d+) \)\); then', line)
    if service and 'mapfile -t packages' in following and 'selected.json' in following:
        service_routes.append((int(service.group(1)), set(re.findall(r'\. == "([^"]+)"', following))))
    cli = re.search(r'if \[\[ "\$SHARD" == (\d+) \]\] && jq -e \'index\("([^"]+)"\)', line)
    if cli:
        cli_routes.append((int(cli.group(1)), cli.group(2)))
    pool = re.search(r'if \(\( SHARD >= (\d+) && SHARD < (\d+) \)\); then', line)
    if pool and 'mapfile -t packages' in following and 'pool.json' in following:
        pool_routes.append((int(pool.group(1)), int(pool.group(2))))
if len(service_routes) != 1 or len(cli_routes) != 1 or len(pool_routes) != 1:
    raise ValueError('special PR worker routes missing or ambiguous')
service_shard, service_packages = service_routes[0]
cli_shard, cli_package = cli_routes[0]
pool_start, pool_end = pool_routes[0]
if not service_packages or cli_package not in routes or pool_start >= pool_end:
    raise ValueError('invalid special PR worker routes')
if set(routes) != dedicated - service_packages - plan_module.GUARD_PACKAGES:
    raise ValueError('PR routes do not match dedicated packages')
workers = plan['include']
seen = {p: [] for p in selected}
for worker in workers:
    shard = worker['shard']
    packages = worker['packages']
    pool = worker['pool']
    browser = any(plan_module.BROWSER.search(package) for package in packages)
    if worker['browser'] is not browser:
        raise ValueError('browser worker evidence does not match its packages')
    if len(packages) != len(set(packages)) or len(pool) != len(set(pool)):
        raise ValueError('duplicate package')
    for package in packages:
        if package not in seen:
            raise ValueError('unselected package in plan')
        seen[package].append(shard)
        if package in pool:
            if package in dedicated or not pool_start <= shard < pool_end:
                raise ValueError('misrouted pool package')
        elif package in routes:
            if shard not in routes[package] and not (package == cli_package and shard == cli_shard):
                raise ValueError('misrouted dedicated package')
        elif package in service_packages:
            if shard != service_shard:
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
        expected = routes[package] | ({cli_shard} if package == cli_package else set())
        if set(shards) != expected or len(shards) != len(expected):
            raise ValueError('dedicated route is incomplete')
    elif package in service_packages:
        if shards != [service_shard]:
            raise ValueError('service route is incomplete')
    elif len(shards) != 1:
        raise ValueError('pool route is incomplete')

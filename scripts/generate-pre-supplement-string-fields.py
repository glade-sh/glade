#!/usr/bin/env python3
"""Generate a local-baseline test fixture from the fail-before Go test output.

Run TestStandardRestrictedPicklistNativeDescribe with the test-only changes on
public commit 471b23c42, capture the output, and pass that file to this script.
The test intentionally fails and logs each field that the baseline typed STRING.
"""
import argparse
import json
import re
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('log', type=Path)
parser.add_argument('--output', type=Path, default=Path('internal/apextest/testdata/conformance/standard_describe_profiles/pre_supplement_string_fields.json'))
args = parser.parse_args()
rows = {'62.0': set(), '67.0': set()}
api = None
for line in args.log.read_text().splitlines():
    subtest = re.search(r'^\s*--- FAIL: TestStandardRestrictedPicklistNativeDescribe/(62\.0|67\.0) ', line)
    if subtest:
        api = subtest.group(1)
        continue
    field = re.search(r':\d+: ([\w.]+): native restricted picklist; glade type STRING', line)
    if field and api:
        rows[api].add(field.group(1))
if any(not values for values in rows.values()):
    raise SystemExit('missing baseline test observations for an API version')
args.output.write_text(json.dumps({k: sorted(v) for k, v in rows.items()}, indent=2) + '\n')

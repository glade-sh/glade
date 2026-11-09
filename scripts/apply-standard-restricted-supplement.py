#!/usr/bin/env python3
"""Add the native restrictedPicklist facet to docs-placeholder picklists.

The broad input carries salesforce-docs-reference placeholder fields with no
restrictedPicklist facet and often no values. The owned P0 capture (a fresh
plain Developer org, default profile, REST describe at API 62.0 and 67.0)
reports these picklists as restricted. For each catalog picklist field on a
docs-reference object that P0 reports restricted at API 67.0, and also at
62.0 when the field exists there, this sets restrictedPicklist. An empty
value list is filled from P0 only when every value is in the reviewed public
allowlist and both APIs list the same values; otherwise it stays empty.
Non-empty catalog lists are kept (the broad input spans more features than a
plain org). A field P0 reports unrestricted at either API stops the script.

DML enforcement of catalog restrictions stays limited to the captured fields
in internal/storage/standard_picklist_validation.go.
"""
import gzip
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1]
storage = root / 'internal/storage'
profiles = root / 'internal/apextest/testdata/conformance/standard_describe_profiles/P0'
with gzip.open(storage / 'standard_describe_catalog.json.gz', 'rt') as f:
    bundle = json.load(f)
native = {}
for api in ('62', '67'):
    with gzip.open(profiles / f'api{api}/default/describes.json.gz', 'rt') as f:
        native[api] = json.load(f)['describes']
with gzip.open(storage / 'testdata/standard_picklist_values.json.gz', 'rt') as f:
    allowed = set(json.load(f))


def native_field(api, obj, name):
    return next((item for item in native[api].get(obj, {}).get('fields', []) if item['name'] == name), None)


def values_of(field):
    return [item.get('value') for item in field.get('picklistValues') or []]


restricted = filled = 0
for obj, describe in sorted(bundle['describes'].items()):
    if describe.get('source') != 'salesforce-docs-reference':
        continue
    for field in describe.get('fields', []):
        if field.get('type') not in ('picklist', 'multipicklist') or field.get('restrictedPicklist'):
            continue
        later, prior = native_field('67', obj, field['name']), native_field('62', obj, field['name'])
        if later is None or not later.get('restrictedPicklist'):
            continue
        if later.get('type') != field['type']:
            raise ValueError(f'{obj}.{field["name"]} type differs from native: {later.get("type")}')
        if prior is not None and not prior.get('restrictedPicklist'):
            raise ValueError(f'{obj}.{field["name"]} is unrestricted at API 62.0')
        field['restrictedPicklist'] = True
        restricted += 1
        entries = later.get('picklistValues') or []
        if field.get('picklistValues') or not entries:
            continue
        if prior is not None and values_of(prior) != values_of(later):
            continue
        if not all(value in allowed for value in values_of(later)):
            continue
        field['picklistValues'] = json.loads(json.dumps(entries))
        filled += 1
path = storage / 'standard_describe_catalog.json.gz'
with path.open('wb') as f:
    with gzip.GzipFile(fileobj=f, mode='wb', filename='', mtime=0) as out:
        out.write(json.dumps(bundle, separators=(',', ':'), ensure_ascii=False).encode())
print(f'restricted {restricted} fields; filled {filled} value lists')

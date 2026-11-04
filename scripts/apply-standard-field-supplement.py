#!/usr/bin/env python3
"""Add native-verified fields to the broad describe input, preserving gates.

Rich fields come from the owned P0 capture; the fresh supplement verifies the
facets used here at both API endpoints. Picklist values use the reviewed public
allowlist rather than values from the scratch org.
"""
import csv
import gzip
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1]
fixture = root / 'internal/apextest/testdata/conformance/standard_field_supplement'
storage = root / 'internal/storage'
with gzip.open(storage / 'standard_describe_catalog.json.gz', 'rt') as f:
    bundle = json.load(f)
with gzip.open(root / 'internal/apextest/testdata/conformance/standard_describe_profiles/P0/api62/granted/describes.json.gz', 'rt') as f:
    prior = json.load(f)['describes']
with gzip.open(storage / 'testdata/standard_picklist_values.json.gz', 'rt') as f:
    allowed = set(json.load(f))
with (fixture / 'supplement.tsv').open() as f:
    rows = list(csv.DictReader(f, delimiter='\t'))
assert len(rows) == 62
for row in rows:
    obj, name = row['object'], row['field']
    source = next(field for field in prior[obj]['fields'] if field['name'] == name)
    for key in ('type', 'length', 'precision', 'scale', 'createable', 'updateable', 'nillable', 'permissionable'):
        if str(source[key]).lower() != row[key].lower():
            raise ValueError(f'{obj}.{name} disagrees with native {row["api"]} {key}')
    fields = bundle['describes'][obj]['fields']
    field = next((item for item in fields if item['name'] == name), None)
    if field is None:
        field = dict(source)
        fields.append(field)
    # An incomplete allowlisted list would block the richer native overlay
    # during placeholder repair. Never present a partial list as complete.
    if 'picklistValues' in source:
        native_values = source['picklistValues']
        field['picklistValues'] = (native_values if all(item.get('value') in allowed for item in native_values) else [])
path = storage / 'standard_describe_catalog.json.gz'
with path.open('wb') as f:
    with gzip.GzipFile(fileobj=f, mode='wb', filename='', mtime=0) as out:
        out.write(json.dumps(bundle, separators=(',', ':'), ensure_ascii=False).encode())

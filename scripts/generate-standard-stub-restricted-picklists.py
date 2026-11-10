#!/usr/bin/env python3
"""Generate the native restricted-picklist supplement for Apex stub fields.

Objects whose field types come from the Apex SObject stub overlay type their
picklists as STRING. The owned P0 capture (a fresh plain Developer org,
default profile, REST describe at API 62.0 and 67.0) reports many of these
fields as restricted picklists. This script writes
internal/storage/standard_stub_restricted_picklists.json: every standard
field P0 reports as a restricted picklist at an API where it exists, with
its native type. A field restricted at one API and unrestricted at the
other, or typed differently, is left out.

A value list is included only when it is complete and safe to carry:
  - the lists are equal (value, label, active, default) at every API where
    the field exists, and non-empty;
  - every label is non-null (Glade renders an absent label as the value);
  - no entry is the default (a default changes DML insert, which P0 does not
    observe);
  - the field is not a dependent picklist (P0 validFor is not carried);
  - every value is in the reviewed public allowlist
    (internal/storage/testdata/standard_picklist_values.json.gz).
Otherwise the list is omitted, because a partial list must never look
complete.

The runtime applies an entry only to a field the stub overlay types STRING
(internal/storage/standard_stub_restricted_picklists.go). The restriction
carries catalog provenance, so DML enforcement stays limited to the captured
fields in internal/storage/standard_picklist_validation.go.
"""
import argparse
import gzip
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--check', action='store_true', help='verify the checked-in catalog without writing it')
parser.add_argument('--output', type=Path, help='write or check a different catalog path')
args = parser.parse_args()
storage = root / 'internal/storage'
profiles = root / 'internal/apextest/testdata/conformance/standard_describe_profiles/P0'
apis = ('62', '67')
native = {}
for api in apis:
    with gzip.open(profiles / f'api{api}/default/describes.json.gz', 'rt') as f:
        native[api] = {
            obj: {field['name']: field for field in describe.get('fields', [])}
            for obj, describe in json.load(f)['describes'].items()
        }
with gzip.open(storage / 'testdata/standard_picklist_values.json.gz', 'rt') as f:
    allowed = set(json.load(f))


def entries_of(field):
    return [
        {'value': item.get('value'), 'label': item.get('label'), 'active': bool(item.get('active')), 'default': bool(item.get('defaultValue'))}
        for item in field.get('picklistValues') or []
    ]


specs = sorted({(obj, name) for api in apis for obj, fields in native[api].items()
                for name, field in fields.items() if field.get('restrictedPicklist')})
out = {}
counts = {'fields': 0, 'values': 0, 'mixed': 0}
for obj, name in specs:
    present = [native[api][obj][name] for api in apis if name in native[api].get(obj, {})]
    if not all(field.get('restrictedPicklist') for field in present) or len({field.get('type') for field in present}) != 1:
        counts['mixed'] += 1
        continue
    kind = present[0]['type']
    if kind not in ('picklist', 'multipicklist'):
        counts['mixed'] += 1
        continue
    entry = {'type': kind}
    lists = [entries_of(field) for field in present]
    values = lists[-1]
    if (values and all(other == values for other in lists)
            and all(item['label'] is not None for item in values)
            and not any(item['default'] for item in values)
            and not any(field.get('dependentPicklist') for field in present)
            and all(item['value'] in allowed for item in values)):
        entry['values'] = values
        counts['values'] += 1
    out.setdefault(obj, {})[name] = entry
    counts['fields'] += 1

doc = {
    'source': 'internal/apextest/testdata/conformance/standard_describe_profiles/P0 (REST describe, API 62.0 and 67.0)',
    'objects': out,
}
path = args.output or storage / 'standard_stub_restricted_picklists.json'
encoder = json.JSONEncoder(separators=(',', ':'), ensure_ascii=False, sort_keys=True)
lines = []
line = ''
width = 0
for token in encoder.iterencode(doc):
    token_width = len(token.encode('utf-8'))
    if token_width > 256:
        raise ValueError('catalog JSON token exceeds the 256-byte line limit')
    if width + token_width > 256:
        lines.append(line)
        line = ''
        width = 0
    line += token
    width += token_width
if line:
    lines.append(line)
generated = '\n'.join(lines) + '\n'
if args.check:
    if not path.is_file() or path.read_text(encoding='utf-8') != generated:
        raise SystemExit('restricted picklist catalog is stale; rerun its generator')
else:
    path.write_text(generated, encoding='utf-8')
print(f"{counts['fields']} restricted picklists; {counts['values']} value lists; {counts['mixed']} left out (restriction or type differs by API)")

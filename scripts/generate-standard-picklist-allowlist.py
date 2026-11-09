#!/usr/bin/env python3
"""Rebuild the reviewed public standard-picklist value allowlist."""
import argparse
import csv
import gzip
import io
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
PUBLIC = 'a recorded revision:internal/storage/standard_describe_catalog.json.gz'
OUTPUT = ROOT / 'internal/storage/testdata/standard_picklist_values.json.gz'
NATIVE = ROOT / 'internal/apextest/testdata/conformance/standard_describe_api68_drift/P0-field-access/org.tsv'
# These separately reviewed additions and their provenance are documented next
# to the output in standard_picklist_values.md.
REVIEWED = {
    'Channel Partner / Reseller', 'Customer - Channel', 'Customer - Direct',
    'Installation Partner', 'Technology Partner',
    'Existing Customer - Downgrade', 'Existing Customer - Replacement',
    'Existing Customer - Upgrade', 'New Customer', 'Partner Referral',
    'Phone Inquiry', 'Purchased List',
}


def generate():
    source = subprocess.run(['git', '-C', str(ROOT), 'show', PUBLIC], check=True, capture_output=True).stdout
    describes = json.loads(gzip.decompress(source))['describes']
    values = set(REVIEWED)
    for obj in describes.values():
        for field in obj.get('fields', []):
            for entry in field.get('picklistValues', []):
                value = entry.get('value')
                if value and not value.endswith(('__c', '__e', '__mdt', '__x')):
                    values.add(value)
    with NATIVE.open(newline='') as stream:
        rows = [row for row in csv.DictReader(stream, delimiter='\t') if row['api'] == '68.0' and row['context'] == 'explicitgrant' and row['route'] == 'named' and row['case'] == 'ShiftAllocationRequest.RequestType.picklist']
    if len(rows) != 1:
        raise ValueError('expected one independently captured picklist observation')
    entries = json.loads(rows[0]['observation'])
    if entries != [{'active': True, 'defaultValue': False, 'label': 'Pick Up', 'value': 'Pickup'}]:
        raise ValueError('captured entry changed; review its public provenance before adding it')
    values.add(entries[0]['value'])
    data = (json.dumps(sorted(values, key=lambda value: value.encode('utf-8')), ensure_ascii=False, separators=(',', ':')) + '\n').encode()
    buffer = io.BytesIO()
    with gzip.GzipFile(fileobj=buffer, mode='wb', filename='', mtime=0) as stream:
        stream.write(data)
    return buffer.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    result = generate()
    if args.check:
        if OUTPUT.read_bytes() != result:
            raise SystemExit('standard picklist allowlist differs')
        print('standard picklist allowlist verified')
    else:
        OUTPUT.write_bytes(result)
        print('standard picklist allowlist generated')


if __name__ == '__main__':
    main()

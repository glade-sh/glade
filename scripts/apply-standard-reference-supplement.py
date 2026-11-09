#!/usr/bin/env python3
"""Replace docs placeholders and add native reference targets.

PLACEHOLDERS names objects whose broad-input entry is a docs-reference
placeholder (no prefix, labels or facets). Each is replaced by the owned P0
API 67.0 describe after checking that API 62.0 lists the same fields with the
same types. Picklist lists follow the reviewed public allowlist, as in
apply-standard-field-supplement.py; origin URLs are not kept.

The owned P0 capture (a fresh plain Developer org, default profile) lists each
target at both API 62.0 and 67.0; plain_global_describe confirms the Task and
Event WhatId targets on the anonymous and isTest routes. Only the targets
named in TARGETS are added, and only to catalog fields that already exist.
"""
import gzip
import json
from pathlib import Path

TARGETS = ['PrivacyRTBFRequest']
# PrivacyPolicyDefinition is PrivacyRTBFRequest.PolicyNameId's target and in the
# native plain global describe (plain_global_describe keys-anonymous-*.txt).
ADDITIONS = ['PrivacyPolicyDefinition']
# Fields native lists at 67.0 but not 62.0.
LATER_FIELDS = {'PrivacyPolicyDefinition': {('IsAllocatePermset', 'boolean')}}
PLACEHOLDERS = ['PrivacyRTBFRequest', 'PrivacyRTBFRequestFeed', 'PrivacyRTBFRequestHistory', 'PrivacyRTBFRequestShare']
# whatid_targets_tiebreak: objects that join the native plain Developer global
# describe after API 62.0. P0 has no API 62.0 describe for them, and native
# lists them as reference targets at 67.0 only (Task and Event WhatId among
# them). The plain Developer object lists keep them out of API 62.0.
LATER_ADDITIONS = ['DevopsEnvDeployment', 'DevopsEnvDeploymentFeed', 'DevopsEnvDeploymentHistory',
                   'DevopsEnvDeploymentMbr', 'DevopsEnvDeploymentMbrFeed', 'DevopsEnvDeploymentMbrHistory']
LATER_PLACEHOLDERS = ['PayGroup', 'PayPeriod']
LATER_TARGETS = ['DevopsEnvDeployment', 'DevopsEnvDeploymentMbr', 'PayGroup', 'PayPeriod', 'SfdcPartnerSbscrOffer']

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
for obj in ADDITIONS + PLACEHOLDERS:
    current = bundle['describes'].get(obj)
    if current is not None and current.get('source') != 'salesforce-docs-reference':
        continue
    if current is None and obj not in ADDITIONS:
        raise ValueError(f'{obj} is not in the broad input')
    source = native['67'][obj]
    shape = lambda describe: sorted((field['name'], field['type']) for field in describe['fields'])
    # The broad input is version-independent. A later field is kept (the 67.0
    # shape); a field 67.0 dropped would be wrong at 67.0 and stops the script.
    later = set(shape(source)) - set(shape(native['62'][obj]))
    if not set(shape(native['62'][obj])) <= set(shape(source)) or later - LATER_FIELDS.get(obj, set()):
        raise ValueError(f'{obj} differs between API 62.0 and 67.0: {sorted(later)}')
    replacement = json.loads(json.dumps(source))
    replacement.pop('urls', None)
    for info in replacement.get('recordTypeInfos', []):
        info.pop('urls', None)
    for field in replacement['fields']:
        values = field.get('picklistValues') or []
        if not all(item.get('value') in allowed for item in values):
            field['picklistValues'] = []
    bundle['describes'][obj] = replacement
def sanitized(source):
    replacement = json.loads(json.dumps(source))
    replacement.pop('urls', None)
    for info in replacement.get('recordTypeInfos', []):
        info.pop('urls', None)
    for field in replacement['fields']:
        values = field.get('picklistValues') or []
        if not all(item.get('value') in allowed for item in values):
            field['picklistValues'] = []
    return replacement
for obj in LATER_ADDITIONS + LATER_PLACEHOLDERS:
    current = bundle['describes'].get(obj)
    if current is not None and current.get('source') != 'salesforce-docs-reference':
        continue
    if (current is None) != (obj in LATER_ADDITIONS):
        raise ValueError(f'{obj}: unexpected broad input entry')
    if obj in native['62'] or obj not in native['67']:
        raise ValueError(f'{obj} is not new at API 67.0 in P0')
    bundle['describes'][obj] = sanitized(native['67'][obj])
added = 0
for obj, describe in sorted(bundle['describes'].items()):
    for field in describe.get('fields', []):
        refs = field.get('referenceTo')
        if refs is None:
            continue
        for target in LATER_TARGETS:
            if target in refs or target not in native['67'] or target in native['62']:
                continue
            source = native['67'].get(obj)
            match = source and next((item for item in source['fields'] if item['name'] == field['name']), None)
            if not match or target not in (match.get('referenceTo') or []):
                continue
            refs.append(target)
            if refs[:-1] == sorted(refs[:-1]):
                refs.sort()
            added += 1
for obj, describe in sorted(bundle['describes'].items()):
    for field in describe.get('fields', []):
        refs = field.get('referenceTo')
        if refs is None:
            continue
        for target in TARGETS:
            if target in refs or target not in bundle['describes']:
                continue
            listed = True
            for api in ('62', '67'):
                source = native[api].get(obj)
                match = source and next((item for item in source['fields'] if item['name'] == field['name']), None)
                if not match or target not in (match.get('referenceTo') or []):
                    listed = False
            if not listed:
                continue
            refs.append(target)
            if refs[:-1] == sorted(refs[:-1]):
                refs.sort()
            added += 1
print(f'added {added} reference targets')
path = storage / 'standard_describe_catalog.json.gz'
with path.open('wb') as f:
    with gzip.GzipFile(fileobj=f, mode='wb', filename='', mtime=0) as out:
        out.write(json.dumps(bundle, separators=(',', ':'), ensure_ascii=False).encode())

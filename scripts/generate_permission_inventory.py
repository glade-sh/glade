#!/usr/bin/env python3
"""Build the local standard permission inventory from owned native captures."""
import csv
import json
from pathlib import Path

root = Path(__file__).resolve().parent.parent
capture = root / 'internal/apextest/testdata/conformance/permission_set_inventory'
links = capture / 'profile-links'
with (capture / 'org.tsv').open() as f:
    rows = [json.loads(r['exact_json'])['record'] for r in csv.DictReader(f, delimiter='\t')
            if r['capture'] == 'a62_anonymous.validated.json' and r['kind'] == 'queryRow'
            and json.loads(r['exact_json'])['queryId'] == 'all']
fresh = json.loads((links / 'permissions-67.json').read_text())['result']['records']
by_name = {r['Name']: r for r in fresh}
user_licenses = {r['Id']: r for r in json.loads((links / 'licenses-67.json').read_text())['result']['records']}
set_licenses = {r['Id']: r for r in json.loads((links / 'permlicenses-67.json').read_text())['result']['records']}
# First map each old-org license to the matching fresh-org license through a
# permission set with a stable native Name. The two remaining profile names
# are org-specific; license-sharing relationships disambiguate them.
license_map = {}
for r in rows:
    if r['Name'] in by_name:
        new = by_name[r['Name']]['LicenseId']
        old = r['LicenseId']
        if old in license_map and license_map[old] != new:
            raise ValueError('native license mapping is ambiguous')
        license_map[old] = new
unmatched = [r for r in fresh if r['Name'] not in {x['Name'] for x in rows}]
for r in rows:
    if r['Name'] in by_name:
        continue
    options = [x for x in unmatched if r['LicenseId'] not in license_map or x['LicenseId'] == license_map[r['LicenseId']]]
    if len(options) != 1:
        continue
    by_name[r['Name']] = options[0]
    license_map[r['LicenseId']] = options[0]['LicenseId']
    unmatched.remove(options[0])
# Once shared-license cases are resolved, exactly one singleton remains.
missing = [r for r in rows if r['Name'] not in by_name]
if len(missing) == len(unmatched) == 1:
    by_name[missing[0]['Name']] = unmatched[0]
    license_map[missing[0]['LicenseId']] = unmatched[0]['LicenseId']
if len(rows) != 88 or any(r['Name'] not in by_name for r in rows):
    raise ValueError('unexpected inventory or profile mapping')
license_ids = sorted(set(r['LicenseId'] for r in rows))
# Profile.UserType per (profile name, user license key) from the fresh-org
# Profile inventory (profile_usertype, API 67.0 anonymous P1).
user_types = {}
with (capture.parent / 'profile_usertype' / 'org.tsv').open() as f:
    for r in csv.DictReader(f, delimiter='\t'):
        if r['api'] == '67.0' and r['route'] == 'anonymous' and r['probe'] == 'P1' and r['record_kind'] == 'ROW':
            row = json.loads(r['native_row_json'])
            user_types[(row['Name'], row['UserLicense']['LicenseDefinitionKey'])] = row['UserType']
licenses = []
for ident in license_ids:
    new = license_map[ident]
    if new in user_licenses:
        src = user_licenses[new]
        licenses.append({'kind': 'UserLicense', 'name': src['Name'], 'key': src['LicenseDefinitionKey']})
    elif new in set_licenses:
        src = set_licenses[new]
        licenses.append({'kind': 'PermissionSetLicense', 'name': src['MasterLabel'], 'key': src['DeveloperName']})
    else:
        raise ValueError('native license is absent from the queried license inventories')
sets = []
for r in rows:
    native = by_name[r['Name']]
    dynamic = r['Name'] != native['Name']
    sets.append({'name': r['Name'] if not dynamic else '', 'label': r['Label'] if not dynamic else '',
                 'type': r['Type'], 'namespace': r['NamespacePrefix'], 'custom': r['IsCustom'],
                 'profile': (native.get('Profile') or {}).get('Name'),
                 'apiOnly': r['PermissionsApiUserOnly'], 'modifyAll': r['PermissionsModifyAllData'],
                 'viewAll': r['PermissionsViewAllData'], 'license': license_ids.index(r['LicenseId'])})
    profile = sets[-1]['profile']
    if profile:
        sets[-1]['userType'] = user_types[(profile, licenses[sets[-1]['license']]['key'])]
out = {'licenses': licenses, 'permissionSets': sets}
(root / 'internal/storage/seeddata/permission_inventory.json').write_text(json.dumps(out, indent=2, ensure_ascii=False) + '\n')

payload = json.loads((capture.parent / 'profile_permission_rows/native-payload/s0-62-anonymous-fields.json').read_text())
field_rows = []
for item in payload:
    if item['kind'] != 'permissionRow':
        continue
    row = item['row']
    profile = row['Parent']['Profile']['Name']
    if profile not in ('System Administrator', 'Standard User'):
        continue
    field_rows.append({'profile': profile, 'object': row['SobjectType'], 'field': row['Field'],
                       'read': row['PermissionsRead'], 'edit': row['PermissionsEdit']})
if len(field_rows) != 94:
    raise ValueError('expected 94 native selected field rows')
(root / 'internal/storage/seeddata/profile_field_permissions.json').write_text(json.dumps(field_rows, indent=2) + '\n')

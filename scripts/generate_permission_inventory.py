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

with (capture.parent / 'profile_field_inventory' / 'org.tsv').open() as f:
    observations = list(csv.DictReader(f, delimiter='\t'))
def field_inventory(api):
    return sorted((r['profile'], r['object'], r['field'], r['read'], r['edit'])
                  for r in observations if r['source_api'] == api)
if field_inventory('62.0') != field_inventory('67.0'):
    raise ValueError('profile field inventory differs by source API')
captured_rows = [dict(profile=profile, object=obj, field=field,
                      read=read == 'true', edit=edit == 'true')
                 for profile, obj, field, read, edit in field_inventory('67.0')
                 if field != '<no rows>']
payload = json.loads((capture.parent / 'profile_permission_rows/native-payload/s0-62-anonymous-fields.json').read_text())
field_rows = []
for item in payload:
    if item['kind'] != 'permissionRow':
        continue
    row = item['row']
    profile = row['Parent']['Profile']['Name']
    if profile in ('System Administrator', 'Standard User'):
        field_rows.append(dict(profile=profile, object=row['SobjectType'], field=row['Field'],
                               read=row['PermissionsRead'], edit=row['PermissionsEdit']))
if len(field_rows) != 94 or any(row not in captured_rows for row in field_rows):
    raise ValueError('original profile field rows no longer match native inventory')
# Existing seeded IDs derive from row position. Keep the original prefix stable.
field_rows.extend(row for row in captured_rows if row not in field_rows)
if len(field_rows) != 224:
    raise ValueError('expected 224 native selected field rows')
(root / 'internal/storage/seeddata/profile_field_permissions.json').write_text(json.dumps(field_rows, indent=2) + '\n')

# Preserve the original profile/object seed order: IDs derive from this index.
object_flags = ('PermissionsRead', 'PermissionsCreate', 'PermissionsEdit',
                'PermissionsDelete', 'PermissionsViewAllRecords', 'PermissionsModifyAllRecords')
original = json.loads((capture.parent / 'profile_account_contact_rows/objperm-62.json').read_text())['result']['records']
object_rows = [{'profile': r['Parent']['Profile']['Name'], 'object': r['SobjectType'],
                'flags': [r[flag] for flag in object_flags]} for r in original]
if len(object_rows) != 56:
    raise ValueError('expected 56 original profile object rows')
with (capture.parent / 'profile_object_inventory/org.tsv').open() as f:
    object_capture = list(csv.DictReader(f, delimiter='\t'))
def object_inventory(api):
    return sorted((r['profile'], r['object'], r['read'], r['create'], r['edit'],
                   r['delete'], r['view_all'], r['modify_all'])
                  for r in object_capture if r['source_api'] == api)
if object_inventory('62.0') != object_inventory('67.0'):
    raise ValueError('profile object inventory differs by source API')
by_object = {(r['profile'], r['object']): r for r in object_rows}
for profile, obj, read, create, edit, delete, view_all, modify_all in object_inventory('67.0'):
    if read == '<no rows>':
        continue
    row = dict(profile=profile, object=obj,
               flags=[v == 'true' for v in (read, create, edit, delete, view_all, modify_all)])
    previous = by_object.get((profile, obj))
    if previous is not None and previous != row:
        raise ValueError(f'profile object permission differs: {profile} {obj}')
    if previous is None:
        object_rows.append(row)
        by_object[(profile, obj)] = row
if len(object_rows) != 66:
    raise ValueError('expected 66 native profile object rows')
(root / 'internal/storage/seeddata/profile_object_permissions.json').write_text(json.dumps(object_rows, indent=0) + '\n')

# A captured user-field axis can constrain an object operation independently
# of its profile's object grant. Derive affected objects from the paired native
# observations instead of listing object names in the runtime.
shape = capture.parent / 'load_defaults_shape'
axis = json.loads((shape / 'axis.json').read_text())
with (shape / 'org.tsv').open() as f:
    observations = list(csv.DictReader(f, delimiter='\t'))
by_shape = {(r['source_api'], r['profile'], r[axis['axis']], r['subject']): r['observation']
            for r in observations if '.' not in r['subject']}
subjects = sorted({key[3] for key in by_shape})
requirements = []
for obj in subjects:
    controls = []
    for api in ('62.0', '67.0'):
        for profile in ('admin', 'standard'):
            before = by_shape[(api, profile, 'false', obj)]
            after = by_shape[(api, profile, 'true', obj)]
            if before not in ('create=true', 'create=false') or after not in ('create=true', 'create=false'):
                raise ValueError(f'invalid object create observation: {obj}')
            controls.append((profile, before, after))
    admin_changes = [(before, after) for profile, before, after in controls if profile == 'admin']
    standard_changes = [(before, after) for profile, before, after in controls if profile == 'standard']
    if len(set(admin_changes)) != 1 or len(set(standard_changes)) != 1:
        raise ValueError(f'object create observations differ by API: {obj}')
    if admin_changes[0] == ('create=false', 'create=true'):
        if standard_changes[0][0] != standard_changes[0][1]:
            raise ValueError(f'unsupported object permission interaction: {obj}')
        requirements.append(dict(object=obj, method=axis['method'], userField=axis['user_field']))
    elif any(before != after for _, before, after in controls):
        raise ValueError(f'unsupported object permission axis change: {obj}')
(root / 'internal/storage/seeddata/object_permission_requirements.json').write_text(json.dumps(requirements, indent=2) + '\n')

Native rows: `internal/apextest/testdata/conformance/profile_field_inventory/org.tsv`

Relative paths in this record refer to `internal/apextest/testdata/conformance/profile_field_inventory/` unless stated otherwise.

Captured 2026-11-09 in a fresh 30-day plain Developer scratch org from a campaign Dev Hub. Used no deployed project metadata, namespace, or feature flags.
At each REST API 62.0 and 67.0 queried FieldPermissions for profile-owned permission sets of the System Administrator and Standard User profiles, restricted to Account, Contact, Lead, Case, Task, Product2, Pricebook2 and Campaign. Selected only profile name, object, field, PermissionsRead and PermissionsEdit. Each version returned 224 rows with done=true; the normalized results were identical. org.tsv records every result plus explicit zero-row groups for the selected profile/object boundaries.
Deleted the scratch org using the CLI and queried its ScratchOrgInfo status through its Dev Hub: Deleted.

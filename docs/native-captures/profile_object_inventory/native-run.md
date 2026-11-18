Native rows: `internal/apextest/testdata/conformance/profile_object_inventory/org.tsv`

Relative paths in this record refer to `internal/apextest/testdata/conformance/profile_object_inventory/` unless stated otherwise.

Captured 2026-11-09 in a fresh 30-day plain Developer scratch org from a campaign Dev Hub. No namespace, feature flags or project metadata were used for this query.
At REST APIs 62.0 and 67.0 queried profile-owned ObjectPermissions for the System Administrator and Standard User profiles and the eight standard objects in org.tsv. Selected only the profile name, object API name, and read/create/edit/delete/view-all/modify-all booleans. Each endpoint returned 14 rows with done=true. org.tsv also records the two selected profile/object groups with no rows.
The scratch org was deleted; its ScratchOrgInfo status was verified as Deleted on the Dev Hub.

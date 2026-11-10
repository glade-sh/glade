Native rows: `internal/apextest/testdata/conformance/load_defaults_permissions/org.tsv`

Captured 2026-11-09 in a fresh, 30-day plain Developer scratch org from a campaign Dev Hub. Synthetic source and the permission set are in project/. No managed namespace or feature flags were added.
Deployed the synthetic classes and permission set with Metadata API 67.0. Class metadata pins API 62.0 and 67.0. Ran admin, standard, adminGrant and standardGrant methods; all eight deliberately fail with CAPTURE observations. org.tsv records each emitted object or field observation, grouped by source API, profile and permission-set assignment. The grant supplies edit access to two standard fields; ungranted fields and profiles provide controls.
Deleted the scratch org using the CLI and queried its ScratchOrgInfo status through its Dev Hub: Deleted.

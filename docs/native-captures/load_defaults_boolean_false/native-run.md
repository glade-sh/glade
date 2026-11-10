Native rows: `internal/apextest/testdata/conformance/load_defaults_boolean_false/org.tsv`

Relative paths in this record refer to `internal/apextest/testdata/conformance/load_defaults_boolean_false/` unless stated otherwise.

Captured 2026-11-09 in a fresh 30-day plain Developer scratch org from a campaign Dev Hub. No namespace, feature flags, or project metadata were added. probe.apex is synthetic and was executed anonymously at API 62.0 and 67.0 with Apex debug logging.
Both runs compiled and succeeded, emitting eight rows each. The false-overload controls record populated-map membership before and after read, then after an explicit false write, alongside a loaded-default control. Four field-access exceptions are retained as rejection controls.
The scratch org was deleted; its ScratchOrgInfo status was verified as Deleted on the Dev Hub.

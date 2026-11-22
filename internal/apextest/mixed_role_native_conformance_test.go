package apextest

import "testing"

// NATIVE setup first testdata/conformance/mixed_role_role_first/org.tsv:2 anonymous 62.0
// NATIVE setup first testdata/conformance/mixed_role_role_first/org.tsv:3 isTest 62.0
// NATIVE setup first testdata/conformance/mixed_role_role_first/org.tsv:4 anonymous 67.0
// NATIVE setup first testdata/conformance/mixed_role_role_first/org.tsv:5 isTest 67.0
// NATIVE data first testdata/conformance/mixed_role_data_first/org.tsv:2 anonymous 62.0
// NATIVE data first testdata/conformance/mixed_role_data_first/org.tsv:3 isTest 62.0
// NATIVE data first testdata/conformance/mixed_role_data_first/org.tsv:4 anonymous 67.0
// NATIVE data first testdata/conformance/mixed_role_data_first/org.tsv:5 isTest 67.0
func TestUserRoleMixedDMLNativeConformance(t *testing.T) {
	t.Run("role first", func(t *testing.T) {
		runInnerTypeNativeConformance(t, "mixed_role_role_first", "RoleFirstTest", "62.0", "67.0")
	})
	t.Run("data first", func(t *testing.T) {
		runInnerTypeNativeConformance(t, "mixed_role_data_first", "DataFirstTest", "62.0", "67.0")
	})
}

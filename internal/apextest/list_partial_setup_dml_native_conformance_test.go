package apextest

import (
	"reflect"
	"testing"
)

// RC2 loopfix2 clusters (b) and (c), each captured in one owned scratch org.

// Database.<op>(List<SObject>, false) on setup objects returns failed rows at
// every API; the single-record form raises through API 66.
func TestListPartialSetupDMLNativeConformance(t *testing.T) {
	runInnerTypeNativeRoutesParts(t, "list_partial_setup_dml", "ListPartialDmlProbeTest", true, []string{"anonymous", "isTest"},
		func(t *testing.T, got, want any) {
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		}, "62.0", "66.0", "67.0", "68.0")
}

func TestProfileLicenseCreateDescribeNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "profile_license_create_describe", "ProfileLicenseProbeTest", "62.0", "65.0", "67.0")
}

func TestStandardUserDefaultLoadDMLNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "standard_user_default_load_dml", "DefaultLoadDmlProbeTest", "62.0", "67.0", "68.0")
}

// Every standard profile without an Account or Contact object permission row
// has no access to either object, whatever its user type.
func TestNoAccountReadProfileUserNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "no_account_read_profile_user", "NoAccountReadProbeTest", "62.0", "67.0")
}

// A deployed profile owns the ObjectPermissions rows of its user license.
func TestCustomProfileUnlistedObjectAccessNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "custom_profile_unlisted_object_access", "UnlistedProfileProbeTest", "62.0", "67.0")
}

// A loadDefaults construction evaluates Text formulas; only a blank result
// reads null.
func TestLoadDefaultsFormulaTextNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "load_defaults_formula_text", "FormulaDefaultProbeTest", "62.0", "67.0")
}

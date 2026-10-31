package apextest

import "testing"

// Supplemental project and anonymous routes for the native rows in
// testdata/conformance/required_name_insert/org.tsv.
func TestRequiredNameInsertNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "required_name_insert", "RequiredNameProbeTest", "62.0", "67.0")
}

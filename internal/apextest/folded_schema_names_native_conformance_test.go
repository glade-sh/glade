package apextest

import "testing"

func TestFoldedSchemaNamesAnonymousAndIsTestNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "folded_schema_names", "SchemaNameProbeTest", "62.0", "67.0")
}

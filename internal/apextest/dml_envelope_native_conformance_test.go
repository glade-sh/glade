package apextest

import (
	"fmt"
	"sort"
	"testing"
)

// Native rows: testdata/conformance/dml_envelope_insert/org.tsv and
// dml_envelope_other/org.tsv (APIs 62.0 and 67.0, anonymous and isTest).
func TestDMLEnvelopeInsertNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "dml_envelope_insert", "DmlEnvelopeInsertProbeTest", compareObservedMap, "62.0", "67.0")
}

func TestDMLEnvelopeOtherNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "dml_envelope_other", "DmlEnvelopeOtherProbeTest", compareObservedMap, "62.0", "67.0")
}

// compareObservedMap reports each differing case by name.
func compareObservedMap(t *testing.T, got, want any) {
	t.Helper()
	actual, actualOK := got.(map[string]any)
	expected, expectedOK := want.(map[string]any)
	if !actualOK || !expectedOK {
		t.Fatalf("invalid observation: native=%v; Glade=%v", want, got)
	}
	keys := make([]string, 0, len(expected))
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if fmt.Sprint(actual[key]) != fmt.Sprint(expected[key]) {
			t.Errorf("%s: native=%v; Glade=%v", key, expected[key], actual[key])
		}
	}
	if len(actual) != len(expected) {
		t.Errorf("case count: native=%d; Glade=%d", len(expected), len(actual))
	}
}

// Native rows: testdata/conformance/json_sobject_scalar_mismatch/org.tsv.
func TestJSONSObjectScalarMismatchNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "json_sobject_scalar_mismatch", "JsonScalarMismatchProbeTest", compareObservedMap, "62.0", "67.0")
}

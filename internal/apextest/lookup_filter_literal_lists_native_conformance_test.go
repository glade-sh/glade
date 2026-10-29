package apextest

import "testing"

func TestLookupFilterLiteralListsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "lookup_filter_literal_lists", "LookupFilterListProbeTest", func(t *testing.T, got, want any) {
		actual, actualOK := got.(map[string]any)
		expected, expectedOK := want.(map[string]any)
		if !actualOK || !expectedOK || len(expected) != 56 || len(actual) != len(expected) {
			t.Fatalf("invalid lookup filter observations: native=%v; Glade=%v", want, got)
		}
		for key, value := range expected {
			t.Run(key, func(t *testing.T) {
				if actual[key] != value {
					t.Fatalf("native=%v; Glade=%v", value, actual[key])
				}
			})
		}
	}, "62.0", "67.0")
}

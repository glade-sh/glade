package apextest

import "testing"

func TestLookupIDWritesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "lookup_id_writes", "MapleLookupIdProbeTest", func(t *testing.T, got, want any) {
		actual, actualOK := got.(map[string]any)
		expected, expectedOK := want.(map[string]any)
		if !actualOK || !expectedOK || len(expected) != 11 || len(actual) != len(expected) {
			t.Fatalf("invalid lookup observations: native=%v; Glade=%v", want, got)
		}
		for operation, value := range expected {
			t.Run(operation, func(t *testing.T) {
				if actual[operation] != value {
					t.Fatalf("native=%v; Glade=%v", value, actual[operation])
				}
			})
		}
	}, "62.0", "67.0")
}

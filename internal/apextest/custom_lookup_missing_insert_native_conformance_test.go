package apextest

import (
	"reflect"
	"sort"
	"testing"
)

func TestCustomLookupMissingInsertNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "custom_lookup_missing_insert", "LookupFinchProbe", func(t *testing.T, got, want any) {
		actual, actualOK := got.(map[string]any)
		expected, expectedOK := want.(map[string]any)
		if !actualOK || !expectedOK || len(expected) != 6 || len(actual) != len(expected) {
			t.Fatalf("incomplete lookup observations: native=%v; Glade=%v", want, got)
		}
		keys := make([]string, 0, len(expected))
		for name := range expected {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			t.Run(name, func(t *testing.T) {
				if !reflect.DeepEqual(actual[name], expected[name]) {
					t.Errorf("native=%q; Glade=%q", expected[name], actual[name])
				}
			})
		}
	}, "62.0", "67.0")
}

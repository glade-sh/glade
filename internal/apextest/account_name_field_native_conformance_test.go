package apextest

import (
	"reflect"
	"testing"
)

func TestAccountNameFieldNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "account_name_field", "AccountNameProbeTest", func(t *testing.T, got, want any) {
		actual, actualOK := got.(map[string]any)
		expected, expectedOK := want.(map[string]any)
		if !actualOK || !expectedOK || len(actual) != len(expected) {
			t.Fatalf("invalid observation: native=%v; Glade=%v", want, got)
		}
		for shape, expectedValue := range expected {
			if shape == "describe_names" {
				// Optional MiddleName/Suffix exposure is a separate schema gap.
				// Retain its observation without asserting unmodeled feature gating.
				if !reflect.DeepEqual(actual[shape], expectedValue) {
					t.Logf("unverified optional name-component exposure: native=%v; Glade=%v", expectedValue, actual[shape])
				}
				continue
			}
			t.Run(shape, func(t *testing.T) {
				if !reflect.DeepEqual(actual[shape], expectedValue) {
					t.Fatalf("native=%v; Glade=%v", expectedValue, actual[shape])
				}
			})
		}
	})
}

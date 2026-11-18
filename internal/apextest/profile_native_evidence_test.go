package apextest

import "testing"

func TestProfileNativeEvidenceRejectsInvalidRecapture(t *testing.T) {
	first := map[string]any{"name": "First", "license": "One"}
	second := map[string]any{"name": "Second", "license": "Two"}
	original := []any{first}
	inventory := []any{first, second}
	cases := []struct {
		name         string
		observations []any
		variable     bool
		wantError    bool
	}{
		{"matching valid capture", []any{[]any{first}}, false, false},
		{"different eligible capture", []any{[]any{second}}, true, false},
		{"different ineligible capture", []any{[]any{map[string]any{"name": "Outside", "license": "Other"}}}, false, true},
		{"different cardinality", []any{[]any{first, second}}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			variable, err := checkProfileNativeEvidence(original, tc.observations, inventory)
			if (err != nil) != tc.wantError || (!tc.wantError && variable != tc.variable) {
				t.Fatalf("variable=%t error=%v; want variable=%t error=%t", variable, err, tc.variable, tc.wantError)
			}
		})
	}
}

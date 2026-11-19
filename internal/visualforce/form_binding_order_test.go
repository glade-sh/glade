package visualforce

import "testing"

// The repeat postback rows retain the later component value. A synthesized
// bare alias must not be overwritten by a client-ID alias in map iteration.
func TestVisualforceActionParamsRepeatedAliasOrdering(t *testing.T) {
	values := map[string]string{
		"j_id0:f1:j_id1:0:m1": "earlier",
		"j_id0:f1:j_id1:1:m1": "later",
		"m1":                  "later",
	}
	allowed := map[string]bool{"m1": true}
	for i := 0; i < 100; i++ {
		if got := visualforceActionParams(values, allowed)["m1"]; got != "later" {
			t.Fatalf("iteration %d: got %q, want later repeated input", i, got)
		}
	}
}

package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Salesforce documents that decimal scale does not change numeric value:
// https://developer.salesforce.com/docs/platform/apex-ref/references/apex-ref/apex-methods-system-decimal.html
func TestCase349DecimalExternalKeyScale(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		equal bool
	}{
		{"1.25", "1.250", true}, {"-2.50", "-2.5000", true}, {"0.00", "-0.000", true},
		{"1.25", "1.251", false}, {"-2.5", "2.50", false},
	} {
		got := storageValuesEqualForVM(storage.Field{}, storage.DecimalValue(tc.a), storage.DecimalValue(tc.b))
		if got != tc.equal {
			t.Errorf("%q vs %q: got %v, want %v", tc.a, tc.b, got, tc.equal)
		}
	}
}

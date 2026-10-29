package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestStandardFieldEvidenceCommerceShapeCertainty(t *testing.T) {
	for _, row := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"unknown", []string{project.UnspecifiedStandardFieldShape}, true},
		{"unknown-with-additive-feature", []string{project.UnspecifiedStandardFieldShape, "MultiCurrency"}, true},
		{"declared-baseline", nil, false},
		{"license-only", []string{"B2BCommerce", "Communities"}, false},
		{"commerce", []string{"B2BCommerce", "Communities", "CommerceEnabled"}, true},
		{"order-management", []string{"OrderManagement"}, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			// Native positive/rejection rows: standard_describe_profiles/P10
			// and P10/license-only org.tsv. Unknown shapes preserve the old
			// source-only constructor surface without asserting native parity.
			if got := StandardObjectConstructorAvailableForFeatures("WebStore", row.features); got != row.want {
				t.Fatalf("WebStore available=%t, want %t", got, row.want)
			}
		})
	}
}

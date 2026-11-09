package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

// Plain and MultiCurrency observations are in org_object_availability/org.tsv
// and org_currency_availability/org.tsv at APIs 62 and 67.
func TestOrgCurrencyAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false}, {"communities", []string{"Communities"}, false},
		{"multi", []string{"MultiCurrency"}, true}, {"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			for _, name := range []string{"CurrencyType", "DatedConversionRate"} {
				EnsureStandardObject(&org, name)
			}
			ApplyProjectOrgShape(&org, tc.features)
			for _, name := range []string{"CurrencyType", "DatedConversionRate"} {
				if got := org.AllowsStandardObject(name); got != tc.want {
					t.Errorf("%s allowed=%v want %v", name, got, tc.want)
				}
				if _, got := org.Objects[name]; got != tc.want {
					t.Errorf("%s present=%v want %v", name, got, tc.want)
				}
			}
			if !org.AllowsStandardObject("Account") {
				t.Error("Account unavailable")
			}
		})
	}
}

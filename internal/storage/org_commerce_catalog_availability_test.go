package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

// Native rows: org_commerce_availability/org.tsv at APIs 62 and 67.
func TestOrgCommerceCatalogAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false}, {"unrelated", []string{"Communities"}, false},
		{"feature", []string{"OrderManagement"}, true},
		{"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			for _, object := range []string{"BuyerGroup", "CartItem", "CartTax"} {
				EnsureStandardObject(&org, object)
			}
			ApplyProjectOrgShape(&org, tc.features)
			for _, object := range []string{"BuyerGroup", "CartItem", "CartTax"} {
				if got := org.AllowsStandardObject(object); got != tc.want {
					t.Errorf("%s allowed=%t want=%t", object, got, tc.want)
				}
				if _, got := org.Objects[object]; got != tc.want {
					t.Errorf("%s present=%t want=%t", object, got, tc.want)
				}
			}
		})
	}
}

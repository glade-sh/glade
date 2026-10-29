package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Declared shapes use org_object_availability/org.tsv:2-13 and
// standard_describe_profiles/P10/{,license-only/}org.tsv. Unspecified shapes
// preserve the source-only surface; they make no additional native claim.
func TestOrgObjectAvailabilityShapeCertainty(t *testing.T) {
	for _, row := range []struct {
		name     string
		features []string
		commerce bool
		quotes   bool
	}{
		{"unspecified", []string{project.UnspecifiedStandardFieldShape}, true, true},
		{"unspecified-additive", []string{project.UnspecifiedStandardFieldShape, "MultiCurrency"}, true, true},
		{"plain", nil, false, false},
		{"license-only", []string{"B2BCommerce", "Communities"}, false, false},
		{"commerce-enabled", []string{"B2BCommerce", "Communities", "CommerceEnabled"}, true, false},
		{"order-management", []string{"OrderManagement"}, true, false},
		{"quotes", []string{"Quotes"}, false, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			org := NewOrgState()
			for _, object := range []string{"WebStore", "Quote", "QuoteLineItem"} {
				EnsureStandardObject(&org, object)
			}
			ApplyProjectOrgShape(&org, row.features)
			for _, object := range []string{"WebStore", "Quote", "QuoteLineItem"} {
				want := row.quotes
				if object == "WebStore" {
					want = row.commerce
				}
				if got := org.AllowsStandardObject(object); got != want {
					t.Errorf("%s availability=%t, want %t", object, got, want)
				}
				if _, got := org.Objects[object]; got != want {
					t.Errorf("%s retained=%t, want %t", object, got, want)
				}
			}
		})
	}
}

func TestOrgObjectAvailabilityShapeCertaintyTransitions(t *testing.T) {
	org := NewOrgState()
	for _, features := range [][]string{nil, {project.UnspecifiedStandardFieldShape}, nil} {
		ApplyProjectOrgShape(&org, features)
		want := len(features) != 0
		for _, object := range []string{"WebStore", "Quote", "QuoteLineItem"} {
			if _, got := org.Objects[object]; got != want {
				t.Errorf("%s retained=%t, want %t after shape %v", object, got, want, features)
			}
		}
	}
}

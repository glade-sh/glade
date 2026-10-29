package storage

import "testing"

// org_object_availability/org.tsv: plain rows 2-5, OrderManagement rows 6-9,
// and quote-setting rows 10-13, at APIs 62/67 on anonymous and @IsTest routes.
func TestOrgObjectAvailabilityNativeProfile(t *testing.T) {
	for _, tc := range []struct{ feature, object string }{
		{"Quotes", "Quote"}, {"Quotes", "QuoteLineItem"}, {"OrderManagement", "WebStore"},
	} {
		t.Run(tc.object, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, tc.object)
			ApplyOrgShape(&org, nil)
			if _, ok := org.Objects[tc.object]; ok {
				t.Fatal("feature-off object remains in shaped org")
			}
			ApplyOrgShape(&org, []string{tc.feature})
			if _, ok := org.Objects[tc.object]; !ok {
				t.Fatal("enabled feature did not restore object")
			}
		})
	}
}

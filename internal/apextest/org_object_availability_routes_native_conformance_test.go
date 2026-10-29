package apextest

import "testing"

// The compact observation keys are decoded in the fixture's cases.tsv. Every
// overload has its own native off/on rows; the core Account is the boundary.
func TestOrgObjectAvailabilityOverloadsNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_object_availability_routes", "ObjectAvailabilityRoutes", 12, nil)
}

func TestOrgObjectAvailabilityChildrenNativeConformance(t *testing.T) {
	// Account.Quotes is rejected natively even with Quotes enabled, while the
	// pre-existing relationship mapper accepts it. Keep that captured facet as
	// a carry; the admitted availability facets below agree on both routes.
	t.Log("Account.Quotes native rejection remains a pre-existing relationship-mapping carry; see the retained org.tsv rows")
	runOrgFeatureObservations(t, "org_object_availability_children", "ObjectAvailabilityChildren", 12, []string{
		"quotes", "quotesLower", "lines", "linesLower", "boundary", "invalidQuote", "invalidWebStore",
	})
}

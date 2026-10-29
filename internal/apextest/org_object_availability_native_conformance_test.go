package apextest

import "testing"

// Each phase differs from the plain Developer shape by one observed feature or
// setting. Other sampled objects remain captured evidence, not inferred gates.
func TestOrgObjectAvailabilityNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_object_availability", "ObjectAvailabilityProbe", 12, []string{
		"Quote.describe", "Quote.type", "Quote.query",
		"QuoteLineItem.describe", "QuoteLineItem.type", "QuoteLineItem.query",
		"WebStore.describe", "WebStore.type", "WebStore.query",
		"Network.describe", "Network.type", "Network.query",
		"Account.describe", "Account.type", "Account.query",
		"AbsentObject__c.describe", "AbsentObject__c.type", "AbsentObject__c.query",
	})
}

package apextest

import "testing"

// org_features_additive/org.tsv: a scratch definition's features list adds to
// the edition. With features=[Communities] the Developer edition still lacks
// Quote, QuoteLineItem, AccountContactRelation and WebStore, exactly as in the
// plain shape, so Glade must not treat those as edition defaults. Network and
// NetworkMember are a recorded carry (see ORG-RUN.txt) and are not asserted.
func TestOrgFeaturesAdditiveNativeConformance(t *testing.T) {
	var facets []string
	for _, object := range []string{"Quote", "QuoteLineItem", "AccountContactRelation", "WebStore", "Account", "AbsentObject__c"} {
		facets = append(facets, object+".describe", object+".type", object+".query")
	}
	runOrgFeatureObservations(t, "org_features_additive", "ObjectAvailabilityProbe", 2, facets)
}

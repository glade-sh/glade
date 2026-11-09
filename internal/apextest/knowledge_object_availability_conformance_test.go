package apextest

import "testing"

// These existing native rows cover absence in plain and unrelated-feature orgs
// at both APIs and both routes. Positive Knowledge rows live in B4 fixtures.
func TestKnowledgeObjectAvailabilityNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_object_availability", "ObjectAvailabilityProbe", 12, []string{
		"Knowledge__ka.describe", "Knowledge__ka.type", "Knowledge__ka.query",
		"Knowledge__kav.describe", "Knowledge__kav.type", "Knowledge__kav.query",
	})
}

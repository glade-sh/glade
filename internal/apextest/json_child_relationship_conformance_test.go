package apextest

import "testing"

func TestJSONChildRelationshipOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "json_child_relationship.json", "JSONChildRelationship", 6)
}

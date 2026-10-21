package apextest

import "testing"

func TestJSONTaskRelationshipOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "json_task_relationship.json", "JSONTaskRelationship", 5)
}

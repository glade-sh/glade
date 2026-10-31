package apextest

import "testing"

// Salesforce observes distinct translated labels for three cyclic collisions
// at both API versions and on anonymous and named routes (org.tsv:2-21).
func TestSObjectJSON3CollisionNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformanceWithLanguage(t, "testdata/conformance/sobject_json3_collision", "SObjectJSON3CollisionProbe", "SOBJECT_COLLISION|", "", "industryA industryB industryC nullBoundary queryRejection", "fr", "value")
}

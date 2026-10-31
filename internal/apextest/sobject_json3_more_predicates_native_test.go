package apextest

import "testing"

func TestSObjectJSON3MorePredicatesNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformanceWithLanguage(t, "testdata/conformance/sobject_json3_more_predicates", "MorePredicateProbe", "MORE_PREDICATE|", "", "stageProspecting stageQualification stageNeeds stageMaster stageMissing childProspecting childQualification parentPredicate", "fr", "value")
}

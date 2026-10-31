package apextest

import "testing"

func TestSObjectJSON3QualifiedNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformanceWithLanguage(t, "testdata/conformance/sobject_json3_qualified", "QualifiedLabelProbe", "QUALIFIED_LABEL|", "", "qualifiedAccount qualifiedOpportunity qualifiedTask statusTask statusOrder statusContract parentOpportunity semiIndustry semiStage semiMissing", "fr", "value")
}

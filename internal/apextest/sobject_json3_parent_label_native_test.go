package apextest

import "testing"

func TestSObjectJSON3ParentLabelNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformanceWithLanguage(t, "testdata/conformance/sobject_json3_parent_label", "ParentLabelProbe", "PARENT_LABEL|", "", "parentTechnology parentBanking parentAgriculture parentNull parentUnaliased parentMissing rootReject", "fr", "value")
}

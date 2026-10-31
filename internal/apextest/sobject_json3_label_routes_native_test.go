package apextest

import "testing"

func TestSObjectJSON3LabelRoutesNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformanceWithLanguage(t, "testdata/conformance/sobject_json3_label_routes", "LabelRoutesProbe", "LABEL_ROUTES|", "", "filterTechnology filterBanking filterAgriculture filterMaster filterMissing filterLike parentTechnology parentBanking parentNull", "fr", "value")
}

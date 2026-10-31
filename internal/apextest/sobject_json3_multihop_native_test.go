package apextest

import "testing"

func TestSObjectJSON3MultiHopNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformanceWithLanguage(t, "testdata/conformance/sobject_json3_multihop", "MultiHopProbe", "MULTIHOP_LABEL|", "", "directTechnology directBanking qualified missing locator child", "fr", "value")
}

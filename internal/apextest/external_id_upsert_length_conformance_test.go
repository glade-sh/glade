package apextest

import "testing"

func TestExternalIDUpsertLengthNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "external_id_upsert_length", "JuniperUpsertProbeTest", compareObservedMap, "62.0", "67.0")
}

package apextest

import "testing"

// Native rows: testdata/conformance/json_sobject_number_{date,datetime,boolean,years}/org.tsv
// and custom_metadata_user_mode_columns/org.tsv (APIs 62.0 and 67.0, anonymous and isTest).
func TestJSONSObjectNumberScalarsNativeConformance(t *testing.T) {
	t.Run("date", func(t *testing.T) {
		runInnerTypeNativeConformanceWithComparison(t, "json_sobject_number_date", "JsonNumberDateProbeTest", compareObservedMap, "62.0", "67.0")
	})
	t.Run("datetime", func(t *testing.T) {
		runInnerTypeNativeConformanceWithComparison(t, "json_sobject_number_datetime", "JsonNumberDatetimeProbeTest", compareObservedMap, "62.0", "67.0")
	})
	t.Run("boolean", func(t *testing.T) {
		runInnerTypeNativeConformanceWithComparison(t, "json_sobject_number_boolean", "JsonNumberBooleanProbeTest", compareObservedMap, "62.0", "67.0")
	})
	t.Run("years", func(t *testing.T) {
		runInnerTypeNativeConformanceWithComparison(t, "json_sobject_number_years", "JsonNumberYearProbeTest", compareObservedMap, "62.0", "67.0")
	})
}

func TestCustomMetadataUserModeColumnsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "custom_metadata_user_mode_columns", "MdtUserModeProbeTest", compareObservedMap, "62.0", "67.0")
}

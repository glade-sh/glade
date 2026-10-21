package apextest

import "testing"

func TestRecordDefaultAndListTypeNativeConformance(t *testing.T) {
	runNativeProjectRoutes(t, "record_default_and_list_type", "RecordDefaultListProbeTest", []string{"62.0", "67.0"}, 1)
}

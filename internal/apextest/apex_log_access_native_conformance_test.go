package apextest

import "testing"

func TestApexLogAccessNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "apex_log_access", "LogAccessProbeTest", "62.0", "67.0")
}

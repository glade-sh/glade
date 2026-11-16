package apextest

import "testing"

// Counter exhaustion text, boundary values and getter aliases at API 62.0 and
// 67.0, anonymous and @IsTest, captured from a scratch org.
func TestLimitsCounterEdgesNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/limits_counter_edges"}.replay(t)
}

func TestSOSLIsTestEdgesNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/sosl_istest_edges"}.replay(t)
}

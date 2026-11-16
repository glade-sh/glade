package apextest

import "testing"

// Counter exhaustion text, boundary values and getter aliases at API 62.0 and
// 67.0, anonymous and @IsTest, captured from a scratch org.
func TestLimitsCounterEdgesNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/limits_counter_edges", skip: map[string]string{
		"co100/anonymous": "anonymous Apex has no local network transport; the mock route covers the counter",
		"co101/anonymous": "anonymous Apex has no local network transport; the mock route covers the counter",
	}}.replay(t)
}

// Row counters over a 60,000-row table: query rows, locator rows and DML rows.
func TestLimitsRowEdgesNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/limits_row_edges"}.replay(t)
}

func TestSOSLIsTestEdgesNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/sosl_istest_edges"}.replay(t)
}

func TestTestBoundaryEdgesNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/test_boundary_edges"}.replay(t)
}

func TestSOSLSearchMatrixNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/sosl_search_matrix", skip: map[string]string{
		"wi_network/anonymous":      "the scratch org has no Networks; the row records that org's error, not a search shape",
		"wi_network_find/anonymous": "the scratch org has no Networks; the row records that org's error, not a search shape",
		"of_0/anonymous":            "equally relevant matches come back in index relevance order, which is not modeled",
		"dc_nested/anonymous":       "a nested SOQL subquery in a SOSL WHERE clause is not implemented yet",
	}}.replay(t)
}

func TestSOSLCandidateCapsNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/sosl_candidate_caps"}.replay(t)
}

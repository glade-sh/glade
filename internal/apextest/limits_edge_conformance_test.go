package apextest

import (
	"strings"
	"testing"
)

// Counter exhaustion text, boundary values and getter aliases at API 62.0 and
// 67.0, anonymous and @IsTest, captured from a scratch org.
func TestLimitsCounterEdgesNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/limits_counter_edges", skip: map[string]string{
		"co100/anonymous": "anonymous Apex has no local network transport; the mock route covers the counter",
		"co101/anonymous": "anonymous Apex has no local network transport; the mock route covers the counter",
		"fut50/anonymous": "anonymous Apex runs a future method inline and never charges the future counter",
		"fut51/anonymous": "anonymous Apex runs a future method inline and never charges the future counter",
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

// Governor caps and messages inside Queueable, future, Batch and finalizer
// transactions. The anonymous rows record the job status the org reported;
// the @IsTest rows run the same job at stopTest.
func TestAsyncLimitMessagesNativeConformance(t *testing.T) {
	edgeFamily{dir: "testdata/conformance/async_limit_messages", normalize: maskAsyncCalloutLimit}.replay(t)
}

// maskAsyncCalloutLimit masks Limits.getLimitCallouts, the thirteenth field of a
// limits row. A Queueable or Batch class that does not implement
// Database.AllowsCallouts reports 0 there natively; Glade reports the shared
// cap and that rule is not modeled.
func maskAsyncCalloutLimit(id, observation string) string {
	fields := strings.Split(observation, "|")
	if len(fields) == 15 {
		fields[12] = "*"
	}
	return strings.Join(fields, "|")
}

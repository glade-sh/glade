package oracle

import (
	"strings"
	"testing"
)

func TestDiffRunsReportsReadableTraceMismatch(t *testing.T) {
	salesforce := OracleRun{
		TestClass:  "AccountOracleTest",
		TestMethod: "createsRecord",
		Status:     OracleStatusPass,
		Events: []OracleEvent{
			{Type: OracleEventMethodCall, Name: "AccountOracleTest.createsRecord", Sequence: 1},
			{Type: OracleEventSOQL, Query: "SELECT Id FROM Account", Sequence: 2},
		},
	}
	local := OracleRun{
		TestClass:  salesforce.TestClass,
		TestMethod: salesforce.TestMethod,
		Status:     OracleStatusPass,
		Events: []OracleEvent{
			{Type: OracleEventMethodCall, Name: "AccountOracleTest.createsRecord", Sequence: 1},
			{Type: OracleEventDML, Operation: "insert", Object: "Account", Sequence: 2},
		},
	}

	diff := DiffRuns(salesforce, local)
	if diff.Outcome != OracleOutcomeTraceMismatch {
		t.Fatalf("outcome = %q, want %q; diff=%#v", diff.Outcome, OracleOutcomeTraceMismatch, diff)
	}
	if len(diff.Details) == 0 || !strings.Contains(diff.Details[0], "event[1]") || !strings.Contains(diff.Details[0], "soql") || !strings.Contains(diff.Details[0], "dml") {
		t.Fatalf("details not readable: %#v", diff.Details)
	}
}

func TestDiffRunsReportsExceptionMismatchBeforeTrace(t *testing.T) {
	salesforce := OracleRun{
		Status:    OracleStatusFail,
		Exception: &OracleException{Type: "System.AssertException", Message: "expected"},
		Events:    []OracleEvent{{Type: OracleEventDebug, Name: "salesforce"}},
	}
	local := OracleRun{
		Status:    OracleStatusFail,
		Exception: &OracleException{Type: "System.NullPointerException", Message: "local"},
		Events:    []OracleEvent{{Type: OracleEventDebug, Name: "local"}},
	}

	diff := DiffRuns(salesforce, local)
	if diff.Outcome != OracleOutcomeExceptionMismatch {
		t.Fatalf("outcome = %q, want %q", diff.Outcome, OracleOutcomeExceptionMismatch)
	}
}

func TestDiffRunsIgnoresRawLogNoise(t *testing.T) {
	salesforce := OracleRun{
		TestClass:  "AccountOracleTest",
		TestMethod: "createsRecord",
		Status:     OracleStatusPass,
		Events: []OracleEvent{{
			Type:     OracleEventSOQL,
			Sequence: 1,
			Query:    "SELECT Id FROM Account",
			Raw:      "13:11:10.0 (123)|SOQL_EXECUTE_BEGIN|SELECT Id FROM Account",
		}},
	}
	local := OracleRun{
		TestClass:  salesforce.TestClass,
		TestMethod: salesforce.TestMethod,
		Status:     OracleStatusPass,
		Events: []OracleEvent{{
			Type:     OracleEventSOQL,
			Sequence: 1,
			Query:    "SELECT Id FROM Account",
			Raw:      "13:11:12.0 (999)|SOQL_EXECUTE_BEGIN|SELECT Id FROM Account",
		}},
	}

	if diff := DiffRuns(salesforce, local); diff.Outcome != OracleOutcomePass {
		t.Fatalf("outcome = %q, want pass; details=%#v", diff.Outcome, diff.Details)
	}
}

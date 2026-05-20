package oracle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeRunRedactsUnstableValuesButPreservesMeaning(t *testing.T) {
	run := OracleRun{
		SchemaVersion: 1,
		Project:       "/tmp/project",
		OrgAlias:      "oaer-probe-lab",
		TestClass:     "AccountOracleTest",
		TestMethod:    "createsRecord",
		Status:        OracleStatusPass,
		Exception: &OracleException{
			Type:    "System.AssertException",
			Message: "System.AssertException: Assertion Failed near 0018c00002ABCDeAAH at 2026-05-20T10:11:12Z",
		},
		Stack: []OracleStackFrame{{
			Symbol: "AccountOracleTest.createsRecord",
			File:   "classes/AccountOracleTest.cls",
			Line:   42,
			Column: 9,
		}},
		DebugPayloads: []OracleDebugPayload{{
			Label: "capture",
			Value: map[string]any{
				"recordId": "0018c00002ABCDeAAH",
				"email":    "test-user-1716210000@example.com",
				"answer":   float64(42),
			},
		}},
		Events: []OracleEvent{
			{Type: OracleEventMethodCall, Name: "AccountOracleTest.createsRecord", Sequence: 7},
			{Type: OracleEventDML, Operation: "insert", Object: "Account", Fields: []string{"Id", "Name"}, Values: map[string]any{"Id": "0018c00002ABCDeAAH", "Name": "Acme"}, Sequence: 8},
			{Type: OracleEventAssert, Name: "assertEquals", Result: true, Sequence: 9},
		},
		FinalRecords: []OracleRecord{{
			Object: "Account",
			ID:     "0018c00002ABCDeAAH",
			Fields: map[string]any{"Id": "0018c00002ABCDeAAH", "Name": "Acme"},
		}},
	}

	normalized := NormalizeRun(run)
	data, err := json.Marshal(normalized)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, unstable := range []string{"0018c00002ABCDeAAH", "2026-05-20T10:11:12Z", "test-user-1716210000@example.com"} {
		if strings.Contains(text, unstable) {
			t.Fatalf("normalized run still contains unstable value %q: %s", unstable, text)
		}
	}
	if normalized.Events[0].Type != OracleEventMethodCall || normalized.Events[1].Object != "Account" || normalized.Events[1].Fields[1] != "Name" {
		t.Fatalf("event meaning changed: %#v", normalized.Events)
	}
	if normalized.Events[2].Result != true {
		t.Fatalf("assert result was not preserved: %#v", normalized.Events[2].Result)
	}
	if normalized.Exception == nil || normalized.Exception.Type != "System.AssertException" {
		t.Fatalf("exception type changed: %#v", normalized.Exception)
	}
	if normalized.Stack[0].Line != 0 || normalized.Stack[0].Column != 0 {
		t.Fatalf("stack line noise not cleared: %#v", normalized.Stack[0])
	}
}

func TestNormalizeRunSortsStateButKeepsEventOrder(t *testing.T) {
	run := OracleRun{
		SchemaVersion: 1,
		Events: []OracleEvent{
			{Type: OracleEventDebug, Name: "third", Sequence: 3},
			{Type: OracleEventDebug, Name: "first", Sequence: 1},
			{Type: OracleEventDebug, Name: "second", Sequence: 2},
		},
		FinalRecords: []OracleRecord{
			{Object: "Contact", ID: "0038c00002AAAABAA5"},
			{Object: "Account", ID: "0018c00002AAAABAA5"},
		},
	}

	normalized := NormalizeRun(run)
	if got := normalized.Events[0].Name + "," + normalized.Events[1].Name + "," + normalized.Events[2].Name; got != "third,first,second" {
		t.Fatalf("event order changed: %s", got)
	}
	if got := normalized.FinalRecords[0].Object; got != "Account" {
		t.Fatalf("final records not sorted: %#v", normalized.FinalRecords)
	}
}

func TestNormalizeRunPreservesDistinctSalesforceIDIdentity(t *testing.T) {
	run := OracleRun{
		Status: OracleStatusPass,
		Events: []OracleEvent{{
			Type: OracleEventDebug,
			Values: map[string]any{
				"first":       "0018c00002AAAA1AAH",
				"firstAgain":  "0018c00002AAAA1AAH",
				"second":      "0018c00002BBBB2AAH",
				"otherPrefix": "0038c00002CCCC3AAH",
			},
		}},
	}

	values := NormalizeRun(run).Events[0].Values
	if values["first"] != values["firstAgain"] {
		t.Fatalf("same id did not normalize to same token: %#v", values)
	}
	if values["first"] == values["second"] {
		t.Fatalf("distinct ids collapsed: %#v", values)
	}
	if !strings.HasPrefix(values["first"].(string), "<sfid:001#") || !strings.HasPrefix(values["otherPrefix"].(string), "<sfid:003#") {
		t.Fatalf("unexpected id tokens: %#v", values)
	}
}

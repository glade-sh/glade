package oracle

import "testing"

func TestParseApexLogBuildsNormalizedEventsAndPayloads(t *testing.T) {
	log := `13:11:10.0 (123)|METHOD_ENTRY|[1]|01p8c000000AAAA|AccountOracleTest.createsRecord()
13:11:10.0 (456)|SOQL_EXECUTE_BEGIN|[2]|Aggregations:0|SELECT Id, Name FROM Account WHERE Id = '0018c00002ABCDeAAH'
13:11:10.0 (789)|DML_BEGIN|[3]|Op:Insert|Type:Account|Rows:1
13:11:10.0 (999)|USER_DEBUG|[4]|DEBUG|OAER_ORACLE:{"label":"afterInsert","value":{"id":"0018c00002ABCDeAAH","ok":true}}
13:11:10.0 (1000)|EXCEPTION_THROWN|[5]|System.AssertException: Assertion Failed`

	obs := ParseApexLog(log)
	if len(obs.Events) != 5 {
		t.Fatalf("events = %#v", obs.Events)
	}
	if obs.Events[0].Type != OracleEventMethodCall || obs.Events[1].Type != OracleEventSOQL || obs.Events[2].Type != OracleEventDML {
		t.Fatalf("event types = %#v", obs.Events)
	}
	if obs.Events[1].Query != "SELECT Id, Name FROM Account WHERE Id = '<sfid:001#1>'" {
		t.Fatalf("query not normalized: %#v", obs.Events[1])
	}
	if len(obs.DebugPayloads) != 1 || obs.DebugPayloads[0].Label != "afterInsert" {
		t.Fatalf("debug payloads = %#v", obs.DebugPayloads)
	}
	if obs.DebugPayloads[0].Value.(map[string]any)["ok"] != true {
		t.Fatalf("debug payload value = %#v", obs.DebugPayloads[0].Value)
	}
}

func TestParseApexLogIgnoredLinesDoNotAffectEventSequence(t *testing.T) {
	log := `13:11:10.0 (123)|METHOD_ENTRY|[1]|AccountOracleTest.createsRecord()
13:11:10.0 (124)|CODE_UNIT_STARTED|[EXTERNAL]|execute_anonymous_apex
13:11:10.0 (456)|SOQL_EXECUTE_BEGIN|[2]|Aggregations:0|SELECT Id FROM Account`

	obs := ParseApexLog(log)
	if len(obs.Events) != 2 {
		t.Fatalf("events = %#v", obs.Events)
	}
	if obs.Events[0].Sequence != 1 || obs.Events[1].Sequence != 2 {
		t.Fatalf("event sequences = %#v", obs.Events)
	}
}

func TestParseSalesforceTestResultJSONMapsTestOutcomes(t *testing.T) {
	raw := []byte(`{
  "status": 0,
  "result": {
    "summary": {"outcome": "Passed"},
    "tests": [{
      "ApexClass": {"Name": "AccountOracleTest"},
      "MethodName": "createsRecord",
      "Outcome": "Pass",
      "RunTime": 17
    }]
  }
}`)
	runs, err := ParseSalesforceTestResultJSON("fixture", "oaer-probe-lab", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %#v", runs)
	}
	if runs[0].Project != "fixture" || runs[0].OrgAlias != "oaer-probe-lab" || runs[0].Status != OracleStatusPass || runs[0].DurationMS != 17 {
		t.Fatalf("run = %#v", runs[0])
	}
}

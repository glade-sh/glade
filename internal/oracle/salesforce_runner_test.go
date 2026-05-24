package oracle

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseApexLogBuildsNormalizedEventsAndPayloads(t *testing.T) {
	log := `13:11:10.0 (123)|METHOD_ENTRY|[1]|01p8c000000AAAA|AccountOracleTest.createsRecord()
13:11:10.0 (456)|SOQL_EXECUTE_BEGIN|[2]|Aggregations:0|SELECT Id, Name FROM Account WHERE Id = '0018c00002ABCDeAAH'
13:11:10.0 (789)|DML_BEGIN|[3]|Op:Insert|Type:Account|Rows:1
13:11:10.0 (999)|USER_DEBUG|[4]|DEBUG|GLADE_ORACLE:{"label":"afterInsert","value":{"id":"0018c00002ABCDeAAH","ok":true}}
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
	runs, err := ParseSalesforceTestResultJSON("fixture", "glade-probe-lab", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %#v", runs)
	}
	if runs[0].Project != "fixture" || runs[0].OrgAlias != "glade-probe-lab" || runs[0].Status != OracleStatusPass || runs[0].DurationMS != 17 {
		t.Fatalf("run = %#v", runs[0])
	}
}

func TestSalesforceRunnerCanAttachRecentApexLogsForFocusedRun(t *testing.T) {
	runner := &fakeSalesforceCommandRunner{}
	runs, err := (SalesforceRunner{CommandRunner: runner}).RunTests(context.Background(), SalesforceRunOptions{
		Project:     "fixture",
		OrgAlias:    "glade-probe-lab",
		Filter:      "AccountOracleTest.createsRecord",
		CaptureLogs: true,
		LogLimit:    3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %#v", runs)
	}
	if len(runs[0].Events) == 0 || runs[0].Events[0].Type != OracleEventSOQL {
		t.Fatalf("events = %#v", runs[0].Events)
	}
	if len(runs[0].DebugPayloads) != 1 || runs[0].DebugPayloads[0].Label != "done" {
		t.Fatalf("debug payloads = %#v", runs[0].DebugPayloads)
	}
	if len(runs[0].RawArtifacts) != 2 || runs[0].RawArtifacts[1].Type != "ApexLog" || runs[0].RawArtifacts[1].Path != "07L000000000001" {
		t.Fatalf("artifacts = %#v", runs[0].RawArtifacts)
	}
	if !runner.sawDataQuery || !runner.sawGetLog {
		t.Fatalf("expected data query and log fetch, calls = %#v", runner.calls)
	}
}

func TestSalesforceRunnerRetriesMissingBatchClasses(t *testing.T) {
	runner := &partialSalesforceCommandRunner{}
	runs, err := (SalesforceRunner{CommandRunner: runner}).RunTests(context.Background(), SalesforceRunOptions{
		Project:  "fixture",
		OrgAlias: "glade-probe-lab",
		Filter:   "GLADE_Oracle_stdlib_misc_11130,GLADE_Oracle_stdlib_misc_11131",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %#v", runs)
	}
	seen := map[string]bool{}
	for _, run := range runs {
		seen[run.TestClass] = true
	}
	if !seen["GLADE_Oracle_stdlib_misc_11130"] || !seen["GLADE_Oracle_stdlib_misc_11131"] {
		t.Fatalf("runs = %#v", runs)
	}
	if runner.apexRunCalls != 2 {
		t.Fatalf("expected batch plus one retry, calls=%d all=%#v", runner.apexRunCalls, runner.calls)
	}
}

type fakeSalesforceCommandRunner struct {
	calls        []string
	sawDataQuery bool
	sawGetLog    bool
}

func (r *fakeSalesforceCommandRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "apex run test"):
		return []byte(`{
  "status": 0,
  "result": {
    "tests": [{
      "ApexClass": {"Name": "AccountOracleTest"},
      "MethodName": "createsRecord",
      "Outcome": "Pass",
      "RunTime": 17,
      "QueueItemId": "709000000000001"
    }]
  }
}`), nil, nil
	case strings.Contains(joined, "data query") && strings.Contains(joined, "ApexLog"):
		r.sawDataQuery = true
		return []byte(`{"result":{"records":[{"Id":"07L000000000001","Operation":"AccountOracleTest.createsRecord","Request":"Api","StartTime":"2026-05-20T12:00:00.000+0000"}]}}`), nil, nil
	case strings.Contains(joined, "apex get log") && strings.Contains(joined, "07L000000000001"):
		r.sawGetLog = true
		return []byte(`13:11:10.0 (456)|SOQL_EXECUTE_BEGIN|[2]|Aggregations:0|SELECT Id FROM Account
13:11:10.0 (999)|USER_DEBUG|[4]|DEBUG|GLADE_ORACLE:{"label":"done","value":{"ok":true}}
13:11:10.0 (123)|METHOD_ENTRY|[1]|01p000000000001|AccountOracleTest.createsRecord()`), nil, nil
	default:
		return nil, []byte("unexpected command"), context.Canceled
	}
}

type partialSalesforceCommandRunner struct {
	calls        []string
	apexRunCalls int
}

func (r *partialSalesforceCommandRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	joined := strings.Join(args, " ")
	r.calls = append(r.calls, name+" "+joined)
	if !strings.Contains(joined, "apex run test") {
		return nil, []byte("unexpected command"), errors.New("unexpected command")
	}
	r.apexRunCalls++
	switch {
	case strings.Contains(joined, "GLADE_Oracle_stdlib_misc_11130") && strings.Contains(joined, "GLADE_Oracle_stdlib_misc_11131"):
		return salesforceTestJSON("GLADE_Oracle_stdlib_misc_11130", "probe_11130"), nil, nil
	case strings.Contains(joined, "GLADE_Oracle_stdlib_misc_11131"):
		return salesforceTestJSON("GLADE_Oracle_stdlib_misc_11131", "probe_11131"), nil, nil
	default:
		return nil, []byte("unexpected class"), errors.New("unexpected class")
	}
}

func salesforceTestJSON(className, methodName string) []byte {
	return []byte(`{
  "status": 0,
  "result": {
    "tests": [{
      "ApexClass": {"Name": "` + className + `"},
      "MethodName": "` + methodName + `",
      "Outcome": "Pass",
      "RunTime": 17
    }]
  }
}`)
}

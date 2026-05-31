package oracle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildAnonymousProbeScriptStructure(t *testing.T) {
	items := []WorkItem{
		{ProbeID: "p1", SurfaceID: "System.String.format", Area: "stdlib", GeneratedClass: "GLADE_Probe_0", MethodName: "m0"},
		{ProbeID: "p'2", SurfaceID: "System.Math.abs", Area: "stdlib", GeneratedClass: "GLADE_Probe_1", MethodName: "m1"},
	}
	script := BuildAnonymousProbeScript(items)

	if strings.Count(script, anonProbeMarker) < 2 {
		t.Fatalf("expected a marker per probe, got script:\n%s", script)
	}
	if !strings.Contains(script, "'p1'") {
		t.Fatalf("probe id p1 not inlined:\n%s", script)
	}
	if !strings.Contains(script, "'p\\'2'") {
		t.Fatalf("single quote not escaped:\n%s", script)
	}
	if !strings.Contains(script, "JSON.serialize(p)") {
		t.Fatalf("payload not serialized:\n%s", script)
	}
	if !strings.Contains(script, "catch (Exception probeEx)") {
		t.Fatalf("probe block missing try/catch:\n%s", script)
	}
}

func TestExtractApexRunLog(t *testing.T) {
	stdout := []byte(`{"status":0,"result":{"success":true,"compiled":true,"logs":"line1\nGLADE_ORACLE:{}\n"}}`)
	logs, err := extractApexRunLog(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs, "GLADE_ORACLE:{}") {
		t.Fatalf("logs not extracted: %q", logs)
	}
}

func TestAnonymousProbeRunsFromLog(t *testing.T) {
	items := []WorkItem{
		{ProbeID: "p1", GeneratedClass: "GLADE_Probe_0", MethodName: "m0"},
		{ProbeID: "p2", GeneratedClass: "GLADE_Probe_1", MethodName: "m1"},
		{ProbeID: "p3", GeneratedClass: "GLADE_Probe_2", MethodName: "m2"},
	}
	p1, _ := json.Marshal(anonProbePayload{ProbeID: "p1", Status: "generated"})
	p2, _ := json.Marshal(anonProbePayload{ProbeID: "p2", Status: "exception", ExceptionType: "NullPointerException", ExceptionMessage: "boom"})
	log := "noise\n" +
		"12:00:00.0 (1)|USER_DEBUG|[1]|ERROR|" + anonProbeMarker + string(p1) + "\n" +
		"12:00:00.0 (2)|USER_DEBUG|[1]|ERROR|" + anonProbeMarker + string(p2) + "\n"

	runs := AnonymousProbeRunsFromLog("proj", "org", items, log)
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs (incl missing p3), got %d", len(runs))
	}

	byClass := map[string]OracleRun{}
	for _, r := range runs {
		byClass[r.TestClass] = r
		if r.Source != "salesforce" {
			t.Fatalf("source = %q", r.Source)
		}
	}
	if byClass["GLADE_Probe_0"].Status != OracleStatusPass {
		t.Fatalf("p1 status = %q", byClass["GLADE_Probe_0"].Status)
	}
	if byClass["GLADE_Probe_0"].TestMethod != "m0" {
		t.Fatalf("p1 method = %q", byClass["GLADE_Probe_0"].TestMethod)
	}
	p2run := byClass["GLADE_Probe_1"]
	if p2run.Status != OracleStatusRuntimeError || p2run.Exception == nil || p2run.Exception.Type != "NullPointerException" {
		t.Fatalf("p2 run = %+v", p2run)
	}
	if byClass["GLADE_Probe_2"].Status != OracleStatusInfrastructureError {
		t.Fatalf("missing p3 should be infra error, got %q", byClass["GLADE_Probe_2"].Status)
	}
}

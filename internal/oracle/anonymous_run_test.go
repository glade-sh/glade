package oracle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildAnonymousProbeScriptResolvesTypes(t *testing.T) {
	items := []WorkItem{
		{ProbeID: "p1", SurfaceID: "System.String.format", Area: "Core stdlib", Namespace: "System", TypeName: "String", GeneratedClass: "C0", MethodName: "m0"},
		{ProbeID: "p'2", SurfaceID: "Approval.ProcessSubmitRequest", Area: "Product namespaces", Namespace: "Approval", TypeName: "ProcessSubmitRequest", GeneratedClass: "C1", MethodName: "m1"},
	}
	script := BuildAnonymousProbeScript(items)

	if strings.Count(script, anonProbeMarker) != 2 {
		t.Fatalf("expected one marker per probe, got script:\n%s", script)
	}
	if strings.Count(script, "Type.forName(") != 2 {
		t.Fatalf("each probe should resolve its type, got:\n%s", script)
	}
	if !strings.Contains(script, "Type.forName('String')") {
		t.Fatalf("system type should use single-arg form:\n%s", script)
	}
	if !strings.Contains(script, "Type.forName('Approval', 'ProcessSubmitRequest')") {
		t.Fatalf("product namespace should use two-arg form:\n%s", script)
	}
	if !strings.Contains(script, `p\'2`) {
		t.Fatalf("single quote not escaped:\n%s", script)
	}
	if !strings.Contains(script, "JSON.serialize(glp)") {
		t.Fatalf("missing payload serialization:\n%s", script)
	}
}

func TestGeneratedProbeClassAsserts(t *testing.T) {
	cls := generatedProbeClass(probeTarget{
		ProbeID: "p1", SurfaceID: "Approval.ProcessSubmitRequest", Area: "Product namespaces",
		Namespace: "Approval", TypeName: "ProcessSubmitRequest", GeneratedClass: "GLADE_C0", MethodName: "probe_0001",
	})
	for _, want := range []string{
		"@IsTest\npublic class GLADE_C0 {",
		"public static void probe_0001()",
		"Type.forName('Approval', 'ProcessSubmitRequest')",
		"System.assertNotEquals(null, glt",
		"JSON.serialize(glp)",
	} {
		if !strings.Contains(cls, want) {
			t.Fatalf("generated class missing %q:\n%s", want, cls)
		}
	}
}

func TestAnonymousChunksRespectsByteBudget(t *testing.T) {
	items := make([]WorkItem, 2000)
	for i := range items {
		items[i] = WorkItem{
			ProbeID:   "probe-with-a-reasonably-long-id-" + string(rune('A'+i%26)),
			SurfaceID: "Namespace.Type.methodName(arg1,arg2,arg3)",
			Area:      "Product namespaces",
		}
	}
	chunks := AnonymousChunks(items, 0)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	total := 0
	for _, chunk := range chunks {
		total += len(chunk)
		if got := len(BuildAnonymousProbeScript(chunk)); got > anonScriptByteBudget {
			t.Fatalf("chunk script %d bytes exceeds budget %d", got, anonScriptByteBudget)
		}
	}
	if total != len(items) {
		t.Fatalf("chunking dropped items: got %d want %d", total, len(items))
	}
}

func TestParseApexRunResultSuccessAndFailureShapes(t *testing.T) {
	ok := []byte(`{"status":0,"result":{"success":true,"compiled":true,"logs":"x GLADE_ORACLE:{} y"}}`)
	res, err := parseApexRunResult(ok)
	if err != nil || !res.Success || !strings.Contains(res.Logs, "GLADE_ORACLE") {
		t.Fatalf("success shape: %+v err=%v", res, err)
	}

	fail := []byte(`{"name":"executeCompileFailure","data":{"success":false,"compiled":false,"compileProblem":"Script too large","logs":""}}`)
	res, err = parseApexRunResult(fail)
	if err != nil {
		t.Fatalf("failure shape err=%v", err)
	}
	if res.Success || res.CompileProblem != "Script too large" {
		t.Fatalf("failure shape not parsed: %+v", res)
	}
}

func TestAnonymousProbeRunsFromResult(t *testing.T) {
	items := []WorkItem{
		{ProbeID: "p1", SurfaceID: "System.String", GeneratedClass: "C0", MethodName: "m0"},
		{ProbeID: "p2", SurfaceID: "System.Math", GeneratedClass: "C1", MethodName: "m1"},
		{ProbeID: "p3", SurfaceID: "Approval.X", GeneratedClass: "C2", MethodName: "m2"},
		{ProbeID: "p4", SurfaceID: "Ghost.Type", GeneratedClass: "C3", MethodName: "m3"},
	}
	p1, _ := json.Marshal(probePayload{ProbeID: "p1", Status: "resolved"})
	p2, _ := json.Marshal(probePayload{ProbeID: "p2", Status: "exception", ExceptionType: "NullPointerException", ExceptionMessage: "boom"})
	p4, _ := json.Marshal(probePayload{ProbeID: "p4", Status: "missing"})
	logs := "noise\n" +
		"...|USER_DEBUG|[1]|ERROR|" + anonProbeMarker + string(p1) + "\n" +
		"...|USER_DEBUG|[1]|ERROR|" + anonProbeMarker + string(p2) + "\n" +
		"...|USER_DEBUG|[1]|ERROR|" + anonProbeMarker + string(p4) + "\n"

	runs := AnonymousProbeRunsFromResult("proj", "org", items, apexRunResult{Success: true, Logs: logs})
	if len(runs) != 4 {
		t.Fatalf("expected 4 runs, got %d", len(runs))
	}
	byClass := map[string]OracleRun{}
	for _, r := range runs {
		byClass[r.TestClass] = r
	}
	if byClass["C0"].Status != OracleStatusPass || byClass["C0"].TestMethod != "m0" {
		t.Fatalf("resolved run = %+v", byClass["C0"])
	}
	if byClass["C1"].Status != OracleStatusRuntimeError || byClass["C1"].Exception == nil || byClass["C1"].Exception.Type != "NullPointerException" {
		t.Fatalf("exception run = %+v", byClass["C1"])
	}
	if byClass["C3"].Status != OracleStatusFail {
		t.Fatalf("missing-type run should fail, got %q", byClass["C3"].Status)
	}
	if byClass["C2"].Status != OracleStatusInfrastructureError {
		t.Fatalf("probe with no observation should be infra error, got %q", byClass["C2"].Status)
	}
}

func TestAnonymousProbeRunsDedupesEchoedMarkers(t *testing.T) {
	items := []WorkItem{{ProbeID: "p1", GeneratedClass: "C0", MethodName: "m0"}}
	p1, _ := json.Marshal(probePayload{ProbeID: "p1", Status: "resolved"})
	// Salesforce echoes the source script and then the USER_DEBUG output, so the
	// marker appears twice; the echo copy has trailing characters.
	logs := "Execute Anonymous: System.debug(LoggingLevel.ERROR, '" + anonProbeMarker + string(p1) + "');\n" +
		"...|USER_DEBUG|[1]|ERROR|" + anonProbeMarker + string(p1) + "\n"
	runs := AnonymousProbeRunsFromResult("proj", "org", items, apexRunResult{Success: true, Logs: logs})
	if len(runs) != 1 {
		t.Fatalf("expected 1 run after dedupe, got %d: %+v", len(runs), runs)
	}
	if runs[0].Status != OracleStatusPass {
		t.Fatalf("run status = %q", runs[0].Status)
	}
}

func TestAnonymousProbeRunsCompileFailureCarriesProblem(t *testing.T) {
	items := []WorkItem{{ProbeID: "p1", GeneratedClass: "C0", MethodName: "m0"}}
	runs := AnonymousProbeRunsFromResult("proj", "org", items, apexRunResult{Success: false, CompileProblem: "Script too large"})
	if len(runs) != 1 || runs[0].Status != OracleStatusInfrastructureError {
		t.Fatalf("runs = %+v", runs)
	}
	if !strings.Contains(runs[0].Exception.Message, "Script too large") {
		t.Fatalf("compile problem not surfaced: %+v", runs[0].Exception)
	}
}

func TestProbeSurfaceTypeFallback(t *testing.T) {
	cases := []struct{ in, ns, typ string }{
		{"List.clone()", "", "List"},
		{"System.assertNotEquals(expected,actual,msg)", "", "System"},
		{"commercepayments.CardPaymentMethodRequest.equals(obj)", "commercepayments", "CardPaymentMethodRequest"},
		{"Account", "", "Account"},
		{"Approval.ProcessSubmitRequest", "Approval", "ProcessSubmitRequest"},
	}
	for _, c := range cases {
		ns, typ := probeSurfaceType(c.in)
		if ns != c.ns || typ != c.typ {
			t.Errorf("probeSurfaceType(%q) = (%q,%q), want (%q,%q)", c.in, ns, typ, c.ns, c.typ)
		}
	}
}

func TestProbeTargetPrefersStructuredFields(t *testing.T) {
	// surfaceID alone would parse type as the member; structured fields win.
	target := probeTargetFromWorkItem(WorkItem{
		SurfaceID: "List.clone()", Namespace: "", TypeName: "List",
	})
	if target.TypeName != "List" || target.Namespace != "" {
		t.Fatalf("structured fields ignored: %+v", target)
	}
}

package debuglog

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSynthesizeTestBuildsSetupAndCall(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("testdata", "subscriber.log"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	source, err := SynthesizeTest(annotated, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"private class TestProcessorRunReproTest",
		"List<Account> setup_accountRows",
		"new Account(Name = 'Acme')",
		"ns.TestProcessor.run();",
		"System.assert(accountRows.size() >= 1",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("source missing %q:\n%s", want, source)
		}
	}
	if strings.Contains(source, deferredWorkLabel()) {
		t.Fatalf("source contains deferred-work label:\n%s", source)
	}
}

func TestSynthesizeReplayBuildsAnonymousSetupAndCall(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("testdata", "subscriber.log"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := SynthesizeReplayWithOptions(annotated, ReplayOptions{MinConfidence: 0.5, SourceIndex: BuildSourceIndex(index)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"List<Account> setup_accountRows",
		"new Account(Name = 'Acme')",
		"insert setup_accountRows;",
		"ns.TestProcessor.run();",
	} {
		if !strings.Contains(plan.Source, want) {
			t.Fatalf("source missing %q:\n%s", want, plan.Source)
		}
	}
	if plan.EntryPoint.ClassName != "TestProcessor" || plan.EntryPoint.Method != "run" {
		t.Fatalf("entry point = %#v, want TestProcessor.run", plan.EntryPoint)
	}
	if len(plan.SetupObjects) != 1 || plan.SetupObjects[0].ObjectName != "Account" {
		t.Fatalf("setup objects = %#v, want Account", plan.SetupObjects)
	}
	if len(plan.Warnings) == 0 || !strings.Contains(strings.Join(plan.Warnings, "\n"), "METHOD_ENTRY") {
		t.Fatalf("warnings = %#v, want METHOD_ENTRY guidance", plan.Warnings)
	}
	if strings.Contains(plan.Source, "@IsTest") || strings.Contains(plan.Source, "private class") {
		t.Fatalf("replay source should be anonymous Apex, got:\n%s", plan.Source)
	}
	if !strings.Contains(plan.Source, "\nList<Account> setup_accountRows") {
		t.Fatalf("replay setup should be top-level anonymous Apex, got:\n%s", plan.Source)
	}
}

func TestSynthesizeReplayRejectsEntryPointBelowMinConfidence(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("testdata", "subscriber.log"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := SynthesizeReplayWithOptions(annotated, ReplayOptions{MinConfidence: 0.99, SourceIndex: BuildSourceIndex(index)})
	if err == nil {
		t.Fatalf("expected replay confidence error, plan=%#v", plan)
	}
}

func TestSynthesizeReplayRejectsLogOnlyEntryPoint(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("testdata", "subscriber.log"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := SynthesizeReplayWithOptions(annotated, ReplayOptions{MinConfidence: 0.5})
	if err == nil {
		t.Fatalf("expected missing source index error, plan=%#v", plan)
	}
	if !strings.Contains(err.Error(), "source index") {
		t.Fatalf("error = %v, want source index", err)
	}
}

func TestSynthesizeReplayUsesExecuteAnonymousSource(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("..", "apexlog", "testdata", "anonymous.log"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := SynthesizeReplay(annotated, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"System.debug(\n    TestProcessor.run()\n);",
		"Replay uses the Execute Anonymous source captured in the log.",
	} {
		if !strings.Contains(plan.Source, want) {
			t.Fatalf("source missing %q:\n%s", want, plan.Source)
		}
	}
	if strings.Contains(plan.Source, "insert setup_accountRows") {
		t.Fatalf("execute anonymous replay should not run inferred setup inserts first:\n%s", plan.Source)
	}
	if plan.EntryPoint.ClassName != "TestProcessor" || plan.EntryPoint.Method != "run" {
		t.Fatalf("entry point = %#v, want TestProcessor.run", plan.EntryPoint)
	}
	if len(plan.SetupObjects) == 0 || plan.SetupObjects[0].ObjectName != "Account" {
		t.Fatalf("setup evidence = %#v, want Account evidence", plan.SetupObjects)
	}
}

func TestSynthesizeReplayWithEntryIndexUsesSelectedMethodFrame(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("..", "apexlog", "testdata", "exception.log"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	entryIndex := -1
	for i, entry := range annotated.Log.Entries {
		if entry.Kind == "EXCEPTION_THROWN" {
			entryIndex = i
			break
		}
	}
	if entryIndex < 0 {
		t.Fatal("missing exception entry")
	}

	plan, err := SynthesizeReplayWithOptions(annotated, ReplayOptions{MinConfidence: 0.5, EntryIndex: entryIndex, UseEntryIndex: true, SourceIndex: BuildSourceIndex(index)})
	if err != nil {
		t.Fatal(err)
	}
	if plan.EntryPoint.ClassName != "TestProcessor" || plan.EntryPoint.Method != "fail" {
		t.Fatalf("entry point = %#v, want TestProcessor.fail", plan.EntryPoint)
	}
	if !strings.Contains(plan.Source, "ns.TestProcessor.fail();") {
		t.Fatalf("source missing fail call:\n%s", plan.Source)
	}
}

func TestSynthesizeReplayEntryIndexRejectsNonReplayableEntry(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("testdata", "subscriber.log"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := SynthesizeReplayWithOptions(annotated, ReplayOptions{MinConfidence: 0.5, EntryIndex: 5, UseEntryIndex: true, SourceIndex: BuildSourceIndex(index)})
	if err == nil {
		t.Fatalf("expected non-replayable entry error, plan=%#v", plan)
	}
	if !strings.Contains(err.Error(), "not replayable") {
		t.Fatalf("error = %v, want not replayable", err)
	}
}

func TestSynthesizeReplayEntryIndexRejectsParameterizedMethod(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[14]|01p000000000001|ns.TestProcessor.withParam(String)",
		"00:00:00.002 (2000000)|METHOD_EXIT|[14]|ns.TestProcessor.withParam(String)",
		"",
	}, "\n"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := SynthesizeReplayWithOptions(annotated, ReplayOptions{MinConfidence: 0.5, EntryIndex: 0, UseEntryIndex: true, SourceIndex: BuildSourceIndex(index)})
	if err == nil {
		t.Fatalf("expected parameterized method to be non-replayable, plan=%#v", plan)
	}
}

func TestSynthesizeReplayEntryIndexRejectsMissingSource(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[2]|01p000000000001|ns.MissingClass.run()",
		"00:00:00.002 (2000000)|METHOD_EXIT|[2]|ns.MissingClass.run()",
		"",
	}, "\n"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := SynthesizeReplayWithOptions(annotated, ReplayOptions{MinConfidence: 0.5, EntryIndex: 0, UseEntryIndex: true, SourceIndex: BuildSourceIndex(index)})
	if err == nil {
		t.Fatalf("expected missing source to be non-replayable, plan=%#v", plan)
	}
}

func TestParseMethodEntrySymbolAcceptsCandidateAndConstructorForms(t *testing.T) {
	ns, typ, method := parseMethodEntrySymbol("|METHOD_ENTRY|ns.TestProcessor.run")
	if ns != "ns" || typ != "TestProcessor" || method != "run" {
		t.Fatalf("candidate symbol = %q %q %q", ns, typ, method)
	}
	ns, typ, method = parseMethodEntrySymbol("00:00:00.001 (1)|CONSTRUCTOR_ENTRY|[2]|01p|ns.TestProcessor.TestProcessor()")
	if ns != "ns" || typ != "TestProcessor" || method != "TestProcessor" {
		t.Fatalf("constructor symbol = %q %q %q", ns, typ, method)
	}
}

func TestSynthesizeTestWrapsLoggedException(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustReadApexLog(t, filepath.Join("..", "apexlog", "testdata", "exception.log"))
	annotated, err := Annotate(log, index, 5)
	if err != nil {
		t.Fatal(err)
	}

	source, err := SynthesizeTest(annotated, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"try {",
		"ns.TestProcessor.fail();",
		"catch (Exception e)",
		"System.assertEquals('System.AuraHandledException', e.getTypeName());",
		"System.assert(e.getMessage().contains('Nope'));",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("source missing %q:\n%s", want, source)
		}
	}
	if strings.Contains(source, deferredWorkLabel()) {
		t.Fatalf("source contains deferred-work label:\n%s", source)
	}
}

func deferredWorkLabel() string {
	return "TO" + "DO:"
}

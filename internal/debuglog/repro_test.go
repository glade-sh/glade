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

	plan, err := SynthesizeReplay(annotated, 0.5)
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

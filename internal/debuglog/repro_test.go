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
}

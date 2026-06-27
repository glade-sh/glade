package apexlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCoreDebugLog(t *testing.T) {
	log := mustReadLog(t, "core.log")

	if log.APIVersion != "64.0" {
		t.Fatalf("api version = %q, want 64.0", log.APIVersion)
	}
	if got := findEntry(log, EntryUserDebug).Data.DebugMessage; got != "start work" {
		t.Fatalf("debug message = %q, want start work", got)
	}
	if got := findEntry(log, EntryDMLBegin).Data.DMLType; got != "Account" {
		t.Fatalf("dml type = %q, want Account", got)
	}
	if got := findEntry(log, EntrySOQLExecuteBegin).Data.SOQLQuery; got != "SELECT Id, Name FROM Account WHERE Name = 'Acme'" {
		t.Fatalf("soql query = %q", got)
	}
	if got := limitByName(log, "Maximum CPU time").Used; got != 7 {
		t.Fatalf("cpu used = %d, want 7", got)
	}
}

func TestParseSOQLQueryContainingPipe(t *testing.T) {
	log := mustReadLog(t, "pipe_query.log")
	if got := findEntry(log, EntrySOQLExecuteBegin).Data.SOQLQuery; got != "SELECT Id FROM Account WHERE Name = 'A|B'" {
		t.Fatalf("pipe query = %q", got)
	}
}

func TestParseExecuteAnonymousSource(t *testing.T) {
	log := mustReadLog(t, "anonymous.log")
	want := "System.debug(\n    TestProcessor.run()\n);"
	if log.AnonymousApex != want {
		t.Fatalf("anonymous apex = %q, want %q", log.AnonymousApex, want)
	}
}

func TestParseExceptionStackFrames(t *testing.T) {
	log := mustReadLog(t, "exception.log")
	entry := findEntry(log, EntryExceptionThrown)
	if entry.Data.ExceptionType != "System.AuraHandledException" {
		t.Fatalf("exception type = %q", entry.Data.ExceptionType)
	}
	if len(entry.Data.StackFrames) != 2 {
		t.Fatalf("stack frames = %d, want 2", len(entry.Data.StackFrames))
	}
	if entry.Data.StackFrames[0].Class != "TestProcessor" || entry.Data.StackFrames[0].Method != "fail" || entry.Data.StackFrames[0].Line != 21 {
		t.Fatalf("first stack frame = %#v", entry.Data.StackFrames[0])
	}
}

func TestParseLimitUsage(t *testing.T) {
	log := mustReadLog(t, "core.log")
	tests := []string{
		"Number of SOQL queries",
		"Maximum CPU time",
		"Maximum heap size",
	}
	for _, name := range tests {
		limit := limitByName(log, name)
		if limit.Name == "" {
			t.Fatalf("limit missing: %q", name)
		}
	}
	if got := limitByName(log, "Maximum CPU time").Used; got != 7 {
		t.Fatalf("cpu used = %d, want 7", got)
	}
}

func mustReadLog(t *testing.T, name string) Log {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	log, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func findEntry(log Log, kind EntryKind) Entry {
	for _, entry := range log.Entries {
		if entry.Kind == kind {
			return entry
		}
	}
	return Entry{}
}

func limitByName(log Log, name string) LimitUsage {
	for _, limit := range log.Limits {
		if limit.Name == name {
			return limit
		}
	}
	return LimitUsage{}
}

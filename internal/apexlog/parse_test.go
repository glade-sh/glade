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

func TestParseEditorEventsAndOffsets(t *testing.T) {
	input := strings.Join([]string{
		"10:12:14.1 (123456)|METHOD_ENTRY|[7]|01p000000000001|AccountService.run(String)",
		"10:12:14.2 (123457)|VARIABLE_SCOPE_BEGIN|[7]|name|String|false|false",
		"10:12:14.3 (123458)|VARIABLE_ASSIGNMENT|[7]|name|\"Acme\"",
		"10:12:14.4 (123459)|STATEMENT_EXECUTE|[8]",
		"10:12:14.5 (123460)|HEAP_ALLOCATE|[9]|Bytes:44",
		"10:12:14.6 (123461)|SOQL_EXECUTE_BEGIN|[10]|Aggregations:0|SELECT Id, Name FROM Account WHERE Name = :name",
		"10:12:14.7 (123462)|SOQL_EXECUTE_END|[10]|Rows:1",
		"10:12:14.8 (123463)|METHOD_EXIT|[7]|AccountService.run(String)",
		"",
	}, "\n")

	log, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Entries) != 8 {
		t.Fatalf("entries = %d, want 8", len(log.Entries))
	}

	method := log.Entries[0]
	if method.Kind != EntryMethodEntry {
		t.Fatalf("method kind = %q, want %q", method.Kind, EntryMethodEntry)
	}
	if method.Line != 1 || method.ByteStart != 0 || method.ByteEnd <= method.ByteStart {
		t.Fatalf("method position = line %d bytes %d-%d", method.Line, method.ByteStart, method.ByteEnd)
	}
	if method.Data.SourceLine != 7 || method.Data.MethodSymbol != "AccountService.run(String)" {
		t.Fatalf("method data = %#v", method.Data)
	}

	scope := log.Entries[1]
	if scope.Kind != EntryVariableScopeBegin {
		t.Fatalf("scope kind = %q, want %q", scope.Kind, EntryVariableScopeBegin)
	}
	if scope.Data.VariableName != "name" || scope.Data.VariableType != "String" {
		t.Fatalf("scope data = %#v", scope.Data)
	}

	assign := log.Entries[2]
	if assign.Kind != EntryVariableAssignment {
		t.Fatalf("assignment kind = %q, want %q", assign.Kind, EntryVariableAssignment)
	}
	if assign.Data.VariableName != "name" || assign.Data.VariableValue != "\"Acme\"" {
		t.Fatalf("assignment data = %#v", assign.Data)
	}

	statement := log.Entries[3]
	if statement.Kind != EntryStatementExecute || statement.Data.SourceLine != 8 {
		t.Fatalf("statement = %#v", statement)
	}

	heap := log.Entries[4]
	if heap.Kind != EntryHeapAllocate || heap.Data.HeapBytes != 44 {
		t.Fatalf("heap = %#v", heap)
	}

	soql := log.Entries[5]
	if soql.Data.SourceLine != 10 || soql.Data.SOQLQuery != "SELECT Id, Name FROM Account WHERE Name = :name" {
		t.Fatalf("soql = %#v", soql.Data)
	}

	exit := log.Entries[7]
	if exit.Kind != EntryMethodExit || exit.Data.MethodSymbol != "AccountService.run(String)" {
		t.Fatalf("exit = %#v", exit)
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

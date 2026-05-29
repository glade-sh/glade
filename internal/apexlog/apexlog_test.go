package apexlog

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFormatHeaderAndFraming(t *testing.T) {
	out := Format(&vm.Result{}, nil, Options{})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	if !strings.HasPrefix(lines[0], "64.0 APEX_CODE,DEBUG;") {
		t.Fatalf("unexpected header line: %q", lines[0])
	}
	mustContainInOrder(t, out,
		"|EXECUTION_STARTED",
		"|CODE_UNIT_STARTED|[EXTERNAL]|execute_anonymous_apex",
		"|CUMULATIVE_LIMIT_USAGE",
		"|CODE_UNIT_FINISHED|execute_anonymous_apex",
		"|EXECUTION_FINISHED",
	)
}

func TestFormatInterleavesDebugWithTrace(t *testing.T) {
	result := &vm.Result{
		Trace: []trace.Event{
			{Name: "apex.statement.declare", Args: map[string]any{"line": 1}},
			{Name: "apex.soql", Args: map[string]any{"query": "SELECT Id FROM Account", "rows": 2, "line": 2}},
			{Name: "apex.dml.insert", Args: map[string]any{"operation": "insert", "rows": 1, "objects": []string{"Account"}, "line": 3}},
		},
		// debug before soql (pos 1) and after everything (pos 3)
		DebugEvents: []vm.DebugEvent{
			{Level: "DEBUG", Message: "before query", TracePos: 1, Line: 2},
			{Level: "INFO", Message: "done", TracePos: 3, Line: 3},
		},
	}

	out := Format(result, nil, Options{})

	mustContainInOrder(t, out,
		"USER_DEBUG|[2]|DEBUG|before query",
		"SOQL_EXECUTE_BEGIN|[2]|Aggregations:0|SELECT Id FROM Account",
		"SOQL_EXECUTE_END|[2]|Rows:2",
		"DML_BEGIN|[3]|Op:Insert|Type:Account|Rows:1",
		"DML_END|[3]",
		"USER_DEBUG|[3]|INFO|done",
	)
}

func TestFormatFatalError(t *testing.T) {
	out := Format(&vm.Result{}, errors.New("System.NullPointerException: boom"), Options{})
	if !strings.Contains(out, "FATAL_ERROR|System.NullPointerException: boom") {
		t.Fatalf("expected FATAL_ERROR line, got:\n%s", out)
	}
}

func TestFormatLimitUsageReflectsResult(t *testing.T) {
	out := Format(&vm.Result{Limits: vm.Limits{Queries: 3, DMLStatements: 1, CPUTimeMS: 42}}, nil, Options{})
	mustContain(t, out, "Number of SOQL queries: 3 out of 100")
	mustContain(t, out, "Number of DML statements: 1 out of 150")
	mustContain(t, out, "Maximum CPU time: 42 out of 10000")
}

func TestFormatSanitizesNewlinesInDebug(t *testing.T) {
	result := &vm.Result{
		DebugEvents: []vm.DebugEvent{{Level: "DEBUG", Message: "line one\nline two", TracePos: 0, Line: 5}},
	}
	out := Format(result, nil, Options{})
	mustContain(t, out, "USER_DEBUG|[5]|DEBUG|line one line two")
	if strings.Contains(out, "line one\nline two") {
		t.Fatalf("debug message newline not sanitized:\n%s", out)
	}
}

func TestFormatDebugLineFallsBackToNearestStatement(t *testing.T) {
	result := &vm.Result{
		Trace: []trace.Event{
			{Name: "apex.statement.declare", Args: map[string]any{"line": 7}},
		},
		// Line unset -> should fall back to the nearest preceding statement line.
		DebugEvents: []vm.DebugEvent{{Level: "DEBUG", Message: "hi", TracePos: 1}},
	}
	out := Format(result, nil, Options{})
	mustContain(t, out, "USER_DEBUG|[7]|DEBUG|hi")
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}

func mustContainInOrder(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	idx := 0
	for _, n := range needles {
		pos := strings.Index(haystack[idx:], n)
		if pos < 0 {
			t.Fatalf("expected %q after position %d, full output:\n%s", n, idx, haystack)
		}
		idx += pos + len(n)
	}
}

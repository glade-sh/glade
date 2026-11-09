package apextest

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// A whole-project compile error lists its diagnostics once per run. Every
// later case keeps the first line and points to the case with the full list.
func TestCompileErrorRunListsDiagnosticsOnce(t *testing.T) {
	full := strings.Join([]string{
		"dependency_ambiguous: declared SFDX package dependency matches multiple local projects: /a, /b",
		"/p/classes/One.cls:2:3: GLADESEMA008: Variable does not exist: A",
		"/p/classes/Two.cls:4:5: GLADESEMA008: Variable does not exist: B",
	}, "\n")
	cases := []TestCase{
		{ClassName: "OneTest", MethodName: "a"},
		{ClassName: "OneTest", MethodName: "b"},
		{ClassName: "TwoTest", MethodName: "c"},
	}
	listed := indexCompileError([]diagnostic.Diagnostic{
		{Severity: diagnostic.Error, Code: "dependency_ambiguous", Message: "declared SFDX package dependency matches multiple local projects: /a, /b"},
		{Severity: diagnostic.Error, Code: "GLADESEMA008", NativeMessage: "Variable does not exist: A", File: "/p/classes/One.cls", Range: &diagnostic.Range{Start: diagnostic.Position{Line: 2, Column: 3}}},
		{Severity: diagnostic.Error, Code: "GLADESEMA008", NativeMessage: "Variable does not exist: B", File: "/p/classes/Two.cls", Range: &diagnostic.Range{Start: diagnostic.Position{Line: 4, Column: 5}}},
	})
	if listed == nil || listed.Error() != full {
		t.Fatalf("indexCompileError = %v, want %q", listed, full)
	}
	run := compileErrorRun(cases, listed, time.Now(), Options{})
	var messages []string
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			if result.Problem == nil || result.Problem.Type != "CompileError" {
				t.Fatalf("case %s.%s problem = %#v", result.ClassName, result.MethodName, result.Problem)
			}
			messages = append(messages, result.Problem.Message)
		}
	}
	if len(messages) != 3 {
		t.Fatalf("messages = %q, want three cases", messages)
	}
	if messages[0] != full {
		t.Fatalf("first message = %q, want the full list", messages[0])
	}
	want := "dependency_ambiguous: declared SFDX package dependency matches multiple local projects: /a, /b\n" +
		"(2 more compile diagnostics; the full list is on OneTest.a)"
	for _, message := range messages[1:] {
		if message != want {
			t.Fatalf("repeated message = %q, want %q", message, want)
		}
	}

	single := compileErrorRun(cases, errors.New("one line"), time.Now(), Options{})
	for _, suite := range single.Suites {
		for _, result := range suite.Cases {
			if result.Problem.Message != "one line" {
				t.Fatalf("single-line message = %q, want it unchanged on every case", result.Problem.Message)
			}
		}
	}
}

// N26: only the dependency diagnostic list is shortened. Snapshot, version,
// runtime, generation, gate and setup errors reach compileErrorRun as other
// error values; every case keeps their complete text. One diagnostic whose
// message spans lines also stays whole.
func TestCompileErrorRunKeepsOtherErrorsWhole(t *testing.T) {
	cases := []TestCase{
		{ClassName: "OneTest", MethodName: "a"},
		{ClassName: "OneTest", MethodName: "b"},
		{ClassName: "TwoTest", MethodName: "c"},
	}
	multiLineDiagnostic := indexCompileError([]diagnostic.Diagnostic{
		{Severity: diagnostic.Error, Code: "GLADESEMA006", NativeMessage: "Invalid type: A\nsecond line of the same diagnostic", File: "/p/classes/One.cls"},
	})
	for name, err := range map[string]error{
		"setup":      errors.New("compile test setup OneTest: parse failed\nline 3: unexpected token"),
		"runtime":    fmt.Errorf("runtime: %w", errors.New("class One: link failed\nmissing method two()")),
		"diagnostic": multiLineDiagnostic,
	} {
		run := compileErrorRun(cases, err, time.Now(), Options{})
		for _, suite := range run.Suites {
			for _, result := range suite.Cases {
				if result.Problem == nil || result.Problem.Message != err.Error() {
					t.Errorf("%s: %s.%s message = %#v, want the full text %q on every case", name, result.ClassName, result.MethodName, result.Problem, err.Error())
				}
			}
		}
	}
}

func TestIndexCompileErrorListsIdenticalDiagnosticsOnce(t *testing.T) {
	ambiguous := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "dependency_ambiguous", Message: "matches multiple local projects: /a, /b"}
	other := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA008", Message: "Variable does not exist: A", File: "/p/One.cls"}
	err := indexCompileError([]diagnostic.Diagnostic{ambiguous, ambiguous, other, ambiguous})
	want := "dependency_ambiguous: matches multiple local projects: /a, /b\n/p/One.cls: GLADESEMA008: Variable does not exist: A"
	if err == nil || err.Error() != want {
		t.Fatalf("indexCompileError = %v, want %q", err, want)
	}
}

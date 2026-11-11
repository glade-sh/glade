package lsp

import (
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// A test diagnostic shows the native test-result message, which omits the
// exception message's trailing spaces (apextest test_failure_whitespace).
func TestTestDiagnosticsUseResultMessage(t *testing.T) {
	diagnostics := TestDiagnostics(testreport.Run{Suites: []testreport.Suite{{
		Name: "ReportShape",
		Cases: []testreport.Case{{
			ClassName:  "ReportShape",
			MethodName: "assertion",
			Status:     testreport.StatusFail,
			Problem: &testreport.Problem{
				Message: "Assertion Failed:   assertion  ",
				Stack:   []testreport.StackFrame{{Symbol: "ReportShape.assertion", File: "ReportShape.cls", Line: 2, Column: 3}},
			},
		}},
	}}})
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if got, want := diagnostics[0].Message, "ReportShape.assertion: Assertion Failed:   assertion"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

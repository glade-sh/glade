package testreport

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Native test-result Message strings omit trailing spaces while keeping
// leading ones (apextest test_failure_whitespace/org.tsv). Every report route
// shows that result message; Problem.Message keeps the raw exception text.
func TestReportsShowNativeResultMessage(t *testing.T) {
	run := Run{Suites: []Suite{{
		Name: "ReportShape",
		Cases: []Case{{
			ClassName:  "ReportShape",
			MethodName: "dmlTwo",
			Status:     StatusFail,
			Problem: &Problem{
				Type:    "System.DmlException",
				Message: "  second  ",
				Stack:   []StackFrame{{Symbol: "ReportShape.dmlTwo", File: "force-app/main/default/classes/ReportShape.cls", Line: 3, Column: 2}},
			},
		}},
	}}}
	if got := run.Suites[0].Cases[0].Problem.ResultMessage(); got != "  second" {
		t.Fatalf("ResultMessage() = %q, want %q", got, "  second")
	}

	var console bytes.Buffer
	if err := WriteConsole(&console, run); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(console.String(), "System.DmlException:   second\n") {
		t.Errorf("console does not show the result message:\n%q", console.String())
	}

	var junit bytes.Buffer
	if err := WriteJUnitXML(&junit, run); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(junit.String(), `message="  second"`) {
		t.Errorf("JUnit does not show the result message:\n%s", junit.String())
	}

	var annotations bytes.Buffer
	if err := WriteGitHubAnnotations(&annotations, run); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(annotations.String(), "::  second\n") {
		t.Errorf("annotation does not show the result message: %q", annotations.String())
	}

	var encoded bytes.Buffer
	if err := WriteJSON(&encoded, run); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Suites []struct {
			Cases []struct {
				Problem Problem `json:"problem"`
			} `json:"cases"`
		} `json:"suites"`
	}
	if err := json.Unmarshal(encoded.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.Suites[0].Cases[0].Problem.Message; got != "  second" {
		t.Errorf("JSON message = %q, want %q", got, "  second")
	}
	if got := run.Suites[0].Cases[0].Problem.Message; got != "  second  " {
		t.Errorf("reporting changed the raw message: %q", got)
	}
}

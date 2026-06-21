package cliui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func TestWriteCheckResultPluralizesSummaryCounts(t *testing.T) {
	var out bytes.Buffer
	if err := WriteCheckResult(&out, CheckResultInfo{
		ProjectRoot: "/tmp/project",
		Types:       1,
		Triggers:    1,
		Objects:     1,
	}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "+") || strings.Contains(got, "╭") {
		t.Fatalf("default check output used a decorative box:\n%s", got)
	}
	for _, want := range []string{"Glade check", "No diagnostics found", "Apex types   1", "Triggers     1", "Objects      1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("check output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "1 types") || strings.Contains(got, "1 triggers") || strings.Contains(got, "1 objects") {
		t.Fatalf("summary line used plural for singular count: %q", got)
	}
}

func TestWriteCheckResultRendersDiagnosticsWithTrySteps(t *testing.T) {
	var out bytes.Buffer
	if err := WriteCheckResult(&out, CheckResultInfo{
		ProjectRoot: "/tmp/project",
		Types:       2,
		ExitCode:    1,
		Diagnostics: []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA002",
			Message:  "unknown type MissingType",
			File:     "/tmp/project/force-app/main/default/classes/Hello.cls",
			Range:    &diagnostic.Range{Start: diagnostic.Position{Line: 7, Column: 5}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Glade check",
		"1 diagnostic found",
		"force-app/main/default/classes/Hello.cls:7:5",
		"error GLADESEMA002 unknown type MissingType",
		"Try:",
		"glade schema load --project .",
		"glade check --project .",
		"Summary:",
		"exit code      1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("check output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/tmp/project/force-app") {
		t.Fatalf("default check output leaked absolute path:\n%s", got)
	}
}

func TestWriteCheckResultReportsWarningOnlyExitCodeZero(t *testing.T) {
	var out bytes.Buffer
	if err := WriteCheckResult(&out, CheckResultInfo{
		ProjectRoot: "/tmp/project",
		Types:       2,
		ExitCode:    0,
		Diagnostics: []diagnostic.Diagnostic{{
			Severity: diagnostic.Warning,
			Code:     "GLADEPERF001",
			Message:  "SOQL query runs inside a loop",
			File:     "/tmp/project/force-app/main/default/classes/Hello.cls",
			Range:    &diagnostic.Range{Start: diagnostic.Position{Line: 7, Column: 5}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"1 diagnostic found",
		"warning GLADEPERF001 SOQL query runs inside a loop",
		"exit code      0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("check output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "exit code      1") {
		t.Fatalf("warning-only output reported failure:\n%s", got)
	}
}

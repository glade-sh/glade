package diagnostic

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteSARIF(t *testing.T) {
	report := Report{Diagnostics: []Diagnostic{{
		Severity: Error,
		Code:     "GLADESEMA002",
		Message:  "Unknown type MissingType",
		File:     "force-app/main/classes/Broken.cls",
		Range:    &Range{Start: Position{Line: 1, Column: 27}},
	}}}
	var out bytes.Buffer
	if err := report.WriteSARIF(&out); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("SARIF was not JSON: %v\n%s", err, out.String())
	}
	if got["version"] != "2.1.0" {
		t.Fatalf("version = %#v", got["version"])
	}
	for _, want := range []string{"GLADESEMA002", "force-app/main/classes/Broken.cls", "Unknown type MissingType", `"level": "error"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("SARIF missing %q:\n%s", want, out.String())
		}
	}
}

func TestWriteGitHubAnnotations(t *testing.T) {
	report := Report{Diagnostics: []Diagnostic{{
		Severity: Warning,
		Code:     "GLADEWARN001",
		Message:  "Name has comma, colon: and newline\nnext",
		File:     "force-app/main/classes/Thing.cls",
		Range:    &Range{Start: Position{Line: 7, Column: 3}},
	}}}
	var out bytes.Buffer
	if err := report.WriteGitHubAnnotations(&out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"::warning file=force-app/main/classes/Thing.cls,line=7,col=3,title=GLADEWARN001::",
		"Name has comma, colon: and newline%0Anext",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("annotation missing %q:\n%s", want, got)
		}
	}
}

func TestWriteGitHubAnnotationsWithoutProperties(t *testing.T) {
	report := Report{Diagnostics: []Diagnostic{{
		Severity: Error,
		Message:  "project-level failure",
	}}}
	var out bytes.Buffer
	if err := report.WriteGitHubAnnotations(&out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "::error::project-level failure\n"; got != want {
		t.Fatalf("annotation = %q, want %q", got, want)
	}
}

package testreport

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteGitHubAnnotations(t *testing.T) {
	var out bytes.Buffer
	if err := WriteGitHubAnnotations(&out, sampleRun()); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"::error file=force-app/main/classes/AccountTest.cls,line=42,col=9,title=System.AssertException::Expected true",
		"::error file=force-app/main/classes/BrokenTest.cls,line=3,col=17,title=ApexParser::Missing ';'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("annotations missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "testCreatesAccount") {
		t.Fatalf("pass case should not emit annotation:\n%s", got)
	}
}

func TestWriteGitHubAnnotationsWithoutProperties(t *testing.T) {
	run := Run{Suites: []Suite{{
		Name: "AdHoc",
		Cases: []Case{{
			Name:    "fails",
			Status:  StatusFail,
			Problem: &Problem{Message: "failed before stack"},
		}},
	}}}
	var out bytes.Buffer
	if err := WriteGitHubAnnotations(&out, run); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "::error title=fail::failed before stack\n"; got != want {
		t.Fatalf("annotation = %q, want %q", got, want)
	}
}

func TestWriteHTML(t *testing.T) {
	var out bytes.Buffer
	if err := WriteHTML(&out, sampleRun(), HTMLReportOptions{Title: "Latest Glade Run"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"<!doctype html>",
		"<title>Latest Glade Run</title>",
		"6 total",
		"AccountTest.testRejectsBlankName",
		"System.AssertException",
		"force-app/main/classes/AccountTest.cls:42",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("HTML missing %q:\n%s", want, got)
		}
	}
}

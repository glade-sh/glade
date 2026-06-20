package testreport

import (
	"bytes"
	"fmt"
	"testing"
)

func TestWriteConsole(t *testing.T) {
	var out bytes.Buffer
	if err := WriteConsole(&out, sampleRun()); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"Glade test",
		"6 selected, 1 passed, 1 failed",
		"Selected: 6",
		"Passed:   1",
		"Failed:   1",
		"AccountTest.testCreatesAccount",
		"AccountTest.testRejectsBlankName",
		"System.AssertException: Expected true",
		"force-app/main/classes/AccountTest.cls:42",
		"Next:",
		"glade test failed",
	} {
		if !bytes.Contains([]byte(got), []byte(want)) {
			t.Fatalf("console output missing %q:\n%s", want, got)
		}
	}
	if bytes.Contains([]byte(got), []byte("+")) || bytes.Contains([]byte(got), []byte("╭")) {
		t.Fatalf("console output used decorative box:\n%s", got)
	}
}

func TestWriteConsoleOmitsLastFailedSuggestionWhenRunPasses(t *testing.T) {
	var out bytes.Buffer
	if err := WriteConsole(&out, passingRun()); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("glade test --watch")) {
		t.Fatalf("console output missing watch suggestion:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte("glade test failed")) {
		t.Fatalf("console output suggested last-failed rerun after passing run:\n%s", got)
	}
}

func TestWriteConsoleOmitsLargePassListing(t *testing.T) {
	run := Run{
		Name:       "fixture",
		DurationMS: 2500,
		Suites: []Suite{{
			Name: "AccountTest",
			Cases: []Case{{
				ClassName:  "AccountTest",
				MethodName: "testCreatesAccount",
				Status:     StatusPass,
				DurationMS: 12,
			}},
		}},
	}
	for i := 0; i < consoleDetailLimit; i++ {
		run.Suites[0].Cases = append(run.Suites[0].Cases, Case{
			ClassName:  "AccountTest",
			MethodName: fmt.Sprintf("testBulk%d", i),
			Status:     StatusPass,
			DurationMS: 1,
		})
	}

	var out bytes.Buffer
	if err := WriteConsole(&out, run); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if bytes.Contains([]byte(got), []byte("testBulk0")) {
		t.Fatalf("expected pass listing to be omitted for large run:\n%s", got)
	}
	for _, want := range []string{
		"... 81 passed tests omitted from listing",
		"3s",
	} {
		if !bytes.Contains([]byte(got), []byte(want)) {
			t.Fatalf("console output missing %q:\n%s", want, got)
		}
	}
}

func TestWriteConsoleWithReportPath(t *testing.T) {
	var out bytes.Buffer
	if err := WriteConsoleWithOptions(&out, sampleRun(), ConsoleOptions{ReportPath: ".glade/runs/latest/summary.md"}); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); !bytes.Contains([]byte(got), []byte("Artifacts:\n  Report  .glade/runs/latest/summary.md\n")) {
		t.Fatalf("console output missing report path:\n%s", got)
	}
}

func TestWriteJSON(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, sampleRun()); err != nil {
		t.Fatal(err)
	}

	const want = `{
  "name": "fixture",
  "summary": {
    "total": 6,
    "passed": 1,
    "failed": 1,
    "skipped": 1,
    "compileErrors": 1,
    "runtimeErrors": 1,
    "unsupported": 1,
    "errors": 3,
    "durationMs": 36
  },
  "suites": [
    {
      "name": "AccountTest",
      "cases": [
        {
          "className": "AccountTest",
          "methodName": "testCreatesAccount",
          "status": "pass",
          "durationMs": 12
        },
        {
          "className": "AccountTest",
          "methodName": "testRejectsBlankName",
          "status": "fail",
          "durationMs": 5,
          "problem": {
            "type": "System.AssertException",
            "message": "Expected true",
            "detail": "expected true but was false",
            "stack": [
              {
                "symbol": "AccountTest.testRejectsBlankName",
                "file": "force-app/main/classes/AccountTest.cls",
                "line": 42,
                "column": 9
              }
            ]
          }
        }
      ]
    },
    {
      "name": "MixedSuite",
      "cases": [
        {
          "className": "BillingTest",
          "methodName": "testGateway",
          "status": "skipped",
          "problem": {
            "message": "feature flag disabled"
          }
        },
        {
          "name": "BrokenTest",
          "className": "BrokenTest",
          "status": "compile_error",
          "durationMs": 7,
          "problem": {
            "type": "ApexParser",
            "message": "Missing ';'",
            "stack": [
              {
                "symbol": "BrokenTest",
                "file": "force-app/main/classes/BrokenTest.cls",
                "line": 3,
                "column": 17
              }
            ]
          }
        },
        {
          "className": "CrashTest",
          "methodName": "testThrows",
          "status": "runtime_error",
          "durationMs": 10,
          "problem": {
            "type": "NullPointerException",
            "message": "Attempt to de-reference a null object"
          }
        },
        {
          "className": "FutureTest",
          "methodName": "testCallout",
          "status": "unsupported",
          "durationMs": 2,
          "problem": {
            "message": "callouts are not supported yet"
          }
        }
      ]
    }
  ]
}
`
	if got := out.String(); got != want {
		t.Fatalf("JSON output mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestWriteJUnitXML(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJUnitXML(&out, sampleRun()); err != nil {
		t.Fatal(err)
	}

	const want = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="fixture" tests="6" failures="1" errors="3" skipped="1" time="0.036">
  <testsuite name="AccountTest" tests="2" failures="1" errors="0" skipped="0" time="0.017">
    <testcase name="testCreatesAccount" classname="AccountTest" time="0.012"></testcase>
    <testcase name="testRejectsBlankName" classname="AccountTest" time="0.005">
      <failure type="System.AssertException" message="Expected true">expected true but was false&#xA;at AccountTest.testRejectsBlankName (force-app/main/classes/AccountTest.cls:42:9)</failure>
    </testcase>
  </testsuite>
  <testsuite name="MixedSuite" tests="4" failures="0" errors="3" skipped="1" time="0.019">
    <testcase name="testGateway" classname="BillingTest" time="0.000">
      <skipped type="skipped" message="feature flag disabled"></skipped>
    </testcase>
    <testcase name="BrokenTest" classname="BrokenTest" time="0.007">
      <error type="ApexParser" message="Missing &#39;;&#39;">at BrokenTest (force-app/main/classes/BrokenTest.cls:3:17)</error>
    </testcase>
    <testcase name="testThrows" classname="CrashTest" time="0.010">
      <error type="NullPointerException" message="Attempt to de-reference a null object"></error>
    </testcase>
    <testcase name="testCallout" classname="FutureTest" time="0.002">
      <error type="unsupported" message="callouts are not supported yet"></error>
    </testcase>
  </testsuite>
</testsuites>
`
	if got := out.String(); got != want {
		t.Fatalf("JUnit XML output mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func sampleRun() Run {
	return Run{
		Name: "fixture",
		Suites: []Suite{
			{
				Name: "AccountTest",
				Cases: []Case{
					{
						ClassName:  "AccountTest",
						MethodName: "testCreatesAccount",
						Status:     StatusPass,
						DurationMS: 12,
					},
					{
						ClassName:  "AccountTest",
						MethodName: "testRejectsBlankName",
						Status:     StatusFail,
						DurationMS: 5,
						Problem: &Problem{
							Type:    "System.AssertException",
							Message: "Expected true",
							Detail:  "expected true but was false",
							Stack: []StackFrame{{
								Symbol: "AccountTest.testRejectsBlankName",
								File:   "force-app/main/classes/AccountTest.cls",
								Line:   42,
								Column: 9,
							}},
						},
					},
				},
			},
			{
				Name: "MixedSuite",
				Cases: []Case{
					{
						ClassName:  "BillingTest",
						MethodName: "testGateway",
						Status:     StatusSkipped,
						Problem:    &Problem{Message: "feature flag disabled"},
					},
					{
						Name:       "BrokenTest",
						ClassName:  "BrokenTest",
						Status:     StatusCompileError,
						DurationMS: 7,
						Problem: &Problem{
							Type:    "ApexParser",
							Message: "Missing ';'",
							Stack: []StackFrame{{
								Symbol: "BrokenTest",
								File:   "force-app/main/classes/BrokenTest.cls",
								Line:   3,
								Column: 17,
							}},
						},
					},
					{
						ClassName:  "CrashTest",
						MethodName: "testThrows",
						Status:     StatusRuntimeError,
						DurationMS: 10,
						Problem: &Problem{
							Type:    "NullPointerException",
							Message: "Attempt to de-reference a null object",
						},
					},
					{
						ClassName:  "FutureTest",
						MethodName: "testCallout",
						Status:     StatusUnsupported,
						DurationMS: 2,
						Problem:    &Problem{Message: "callouts are not supported yet"},
					},
				},
			},
		},
	}
}

func passingRun() Run {
	return Run{
		Name: "fixture",
		Suites: []Suite{{
			Name: "AccountTest",
			Cases: []Case{{
				ClassName:  "AccountTest",
				MethodName: "testCreatesAccount",
				Status:     StatusPass,
				DurationMS: 12,
			}},
		}},
	}
}

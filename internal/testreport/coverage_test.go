package testreport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func coverageRun(gatePassed bool) Run {
	return Run{
		Name: "glade test",
		Suites: []Suite{{Name: "ShapeTest", Cases: []Case{{
			ClassName: "ShapeTest", MethodName: "left", Status: StatusPass,
			Coverage: []TestCoverage{{Name: "Shape", Type: "class", NumLinesCovered: 2, NumLinesUncovered: 1, CoveredLines: []int{2, 3}}},
		}}}},
		Coverage: &Coverage{
			Summary: CoverageSummary{TotalLines: 7, CoveredLines: 2, TestRunCoverage: 29, OrgWideTotalLines: 7, OrgWideCovered: 2, OrgWideCoverage: 28},
			Components: []ComponentCoverage{
				{Name: "Shape", Type: "class", TotalLines: 3, TotalCovered: 2, CoveredPercent: 67, CoveredLines: []int{2, 3}, UncoveredLines: []int{4}},
				{Name: "ShapeTrigger", Type: "trigger", TotalLines: 4, TotalCovered: 0, CoveredPercent: 0, CoveredLines: []int{}, UncoveredLines: []int{2, 3, 4, 6}},
			},
			Gate: &CoverageGate{TestLevel: CoverageGateRunLocalTests, Percent: 28, Passed: gatePassed},
		},
	}
}

func TestCoverageJSONRoundTrip(t *testing.T) {
	run := coverageRun(true)
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"coverage":{"summary":{"totalLines":7`, `"testRunCoverage":29`, `"orgWideCoverage":28`, `"coveredLines":[2,3]`, `"numLinesUncovered":1`, `"gate":{"testLevel":"RunLocalTests"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("JSON missing %s:\n%s", want, data)
		}
	}
	var decoded Run
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Coverage, run.Coverage) || !reflect.DeepEqual(decoded.Suites[0].Cases[0].Coverage, run.Suites[0].Cases[0].Coverage) {
		t.Fatalf("round trip changed coverage:\n%+v", decoded.Coverage)
	}
	plain := run
	plain.Coverage = nil
	plain.Suites = []Suite{{Name: "ShapeTest", Cases: []Case{{ClassName: "ShapeTest", MethodName: "left"}}}}
	data, err = json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "coverage") {
		t.Fatalf("a run without coverage writes a coverage field:\n%s", data)
	}
}

// A failed coverage gate fails the run, like a failed test.
func TestCoverageGateFailureFailsSummary(t *testing.T) {
	if s := coverageRun(true).Summary(); !s.OK() || s.CoverageGateFailed {
		t.Fatalf("passing gate summary = %+v", s)
	}
	if s := coverageRun(false).Summary(); s.OK() || !s.CoverageGateFailed {
		t.Fatalf("failing gate summary = %+v", s)
	}
	noGate := coverageRun(true)
	noGate.Coverage.Gate = nil
	if s := noGate.Summary(); !s.OK() {
		t.Fatalf("coverage without a gate summary = %+v", s)
	}
}

func TestCoverageConsoleAndJUnit(t *testing.T) {
	run := coverageRun(false)
	run.Coverage.Gate.Warnings = []string{"Average test coverage across all Apex Classes and Triggers is 28%, at least 75% test coverage is required."}
	var console bytes.Buffer
	if err := WriteConsole(&console, run); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Code coverage: 29% of 7 lines in this run (2 covered), 28% org-wide", "Shape          67%  2/3  uncovered 4", "ShapeTrigger    0%  0/4  uncovered 2,3,4,6", "Coverage gate RunLocalTests: failed (28%)", "Average test coverage across all Apex Classes"} {
		if !strings.Contains(console.String(), want) {
			t.Fatalf("console missing %q:\n%s", want, console.String())
		}
	}
	var junit bytes.Buffer
	if err := WriteJUnitXML(&junit, run); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(junit.String(), `<property name="testRunCoverage" value="29%"></property>`) || !strings.Contains(junit.String(), `<property name="orgWideCoverage" value="28%"></property>`) {
		t.Fatalf("junit missing coverage properties:\n%s", junit.String())
	}
	if !strings.Contains(junit.String(), `<property name="coverageGate" value="RunLocalTests:failed"></property>`) || !strings.Contains(junit.String(), `<property name="coverageGateWarning" value="Average test coverage across all Apex Classes`) {
		t.Fatalf("junit missing coverage gate properties:\n%s", junit.String())
	}
	run.Coverage = nil
	junit.Reset()
	if err := WriteJUnitXML(&junit, run); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(junit.String(), "properties") {
		t.Fatalf("junit without coverage has properties:\n%s", junit.String())
	}
}

func TestParseCoverageGate(t *testing.T) {
	for in, want := range map[string]string{"": "", "runlocaltests": CoverageGateRunLocalTests, "RunSpecifiedTests": CoverageGateRunSpecifiedTests} {
		if got, err := ParseCoverageGate(in); err != nil || got != want {
			t.Fatalf("ParseCoverageGate(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParseCoverageGate("RunAllTests"); err == nil {
		t.Fatal("RunAllTests accepted")
	}
}

// NATIVE per-test carry-over (test_runner_coverage_suite/org.tsv): in method
// name order a later test's row holds the declaration lines earlier tests
// covered, a class an earlier test reached keeps a row, and every trigger has
// a row in every test.
func TestNativePerTestRowsCarriesDeclarationLines(t *testing.T) {
	run := Run{
		Suites: []Suite{{Name: "T", Cases: []Case{
			{ClassName: "T", MethodName: "beta", Coverage: []TestCoverage{{Name: "C", Type: "class", CoveredLines: []int{5, 6}}}},
			{ClassName: "T", MethodName: "Alpha", Coverage: []TestCoverage{{Name: "C", Type: "class", CoveredLines: []int{2, 3}}}},
			{ClassName: "T", MethodName: "gamma"},
		}}},
		Coverage: &Coverage{Synchronous: true, Components: []ComponentCoverage{
			{Name: "C", Type: "class", TotalLines: 4, CoveredLines: []int{2, 3, 5, 6}, Declaration: []int{2, 5}},
			{Name: "Trg", Type: "trigger", TotalLines: 2, UncoveredLines: []int{2, 3}},
		}},
	}
	got := NativePerTestRows(run)
	rows := map[string][]TestCoverage{}
	for _, c := range got.Suites[0].Cases {
		rows[c.MethodName] = c.Coverage
	}
	if fmt.Sprint(rows["Alpha"]) != "[{C class  2 2 [2 3]} {Trg trigger  0 2 []}]" {
		t.Fatalf("Alpha rows %v", rows["Alpha"])
	}
	if fmt.Sprint(rows["beta"]) != "[{C class  3 1 [2 5 6]} {Trg trigger  0 2 []}]" {
		t.Fatalf("beta rows %v", rows["beta"])
	}
	if fmt.Sprint(rows["gamma"]) != "[{C class  2 2 [2 5]} {Trg trigger  0 2 []}]" {
		t.Fatalf("gamma rows %v", rows["gamma"])
	}
	if len(run.Suites[0].Cases[0].Coverage) != 1 {
		t.Fatal("NativePerTestRows changed its input")
	}
}

// A case whose method never ran (compile error, skipped, or kept from running
// by a failed @TestSetup) has no per-test rows and does not feed the carry.
func TestNativePerTestRowsSkipsCasesThatNeverRan(t *testing.T) {
	run := Run{
		Suites: []Suite{{Name: "T", Cases: []Case{
			{ClassName: "T", MethodName: "alpha", Status: StatusCompileError, CoverageNotRun: true},
			{ClassName: "T", MethodName: "beta", Coverage: []TestCoverage{{Name: "C", Type: "class", CoveredLines: []int{5}}}},
			{ClassName: "T", MethodName: "gamma", Status: StatusFail, CoverageNotRun: true},
		}}},
		Coverage: &Coverage{Synchronous: true, Components: []ComponentCoverage{
			{Name: "C", Type: "class", TotalLines: 2, CoveredLines: []int{5}, UncoveredLines: []int{2}, Declaration: []int{2, 5}},
			{Name: "Trg", Type: "trigger", TotalLines: 1, UncoveredLines: []int{2}},
		}},
	}
	got := NativePerTestRows(run)
	for _, c := range got.Suites[0].Cases {
		switch c.MethodName {
		case "alpha", "gamma":
			if c.Coverage != nil || c.CoverageNotRun {
				t.Fatalf("%s: rows %v notRun %v", c.MethodName, c.Coverage, c.CoverageNotRun)
			}
		case "beta":
			if fmt.Sprint(c.Coverage) != "[{C class  1 1 [5]} {Trg trigger  0 1 []}]" {
				t.Fatalf("beta rows %v", c.Coverage)
			}
		}
	}
}

// Same-named classes from two source units are matched by file: a row maps
// onto its own component, and two same-named test classes in one suite keep
// separate carries.
func TestNativePerTestRowsMatchesComponentsByFile(t *testing.T) {
	run := Run{
		Suites: []Suite{{Name: "T", Cases: []Case{
			{ClassName: "T", MethodName: "a", SelectedSourceFile: "1/T.cls", Coverage: []TestCoverage{{Name: "C", Type: "class", File: "1/C.cls", CoveredLines: []int{2}}}},
			{ClassName: "T", MethodName: "b", SelectedSourceFile: "2/T.cls", Coverage: []TestCoverage{{Name: "C", Type: "class", File: "2/C.cls", CoveredLines: []int{3}}}},
		}}},
		Coverage: &Coverage{Synchronous: true, Components: []ComponentCoverage{
			{Name: "C", Type: "class", File: "1/C.cls", TotalLines: 2, CoveredLines: []int{2}, UncoveredLines: []int{3}, Declaration: []int{2}},
			{Name: "C", Type: "class", File: "2/C.cls", TotalLines: 2, CoveredLines: []int{3}, UncoveredLines: []int{2}, Declaration: []int{3}},
		}},
	}
	got := NativePerTestRows(run)
	rows := map[string]string{}
	for _, c := range got.Suites[0].Cases {
		rows[c.MethodName] = fmt.Sprint(c.Coverage)
	}
	if rows["a"] != "[{C class 1/C.cls 1 1 [2]}]" || rows["b"] != "[{C class 2/C.cls 1 1 [3]}]" {
		t.Fatalf("rows %v", rows)
	}
}

// NATIVE an asynchronous run lists no uncalled trigger; a trigger a test
// reached keeps a row in later tests (test_runner_coverage_carry
// setup-all-async and f7 rows).
func TestNativePerTestRowsAsynchronousRunListsReachedTriggersOnly(t *testing.T) {
	run := Run{
		Suites: []Suite{{Name: "T", Cases: []Case{
			{ClassName: "T", MethodName: "a", Coverage: []TestCoverage{{Name: "Fired", Type: "trigger", CoveredLines: []int{2}}}},
			{ClassName: "T", MethodName: "b"},
		}}},
		Coverage: &Coverage{Components: []ComponentCoverage{
			{Name: "Fired", Type: "trigger", TotalLines: 2, CoveredLines: []int{2}, UncoveredLines: []int{3}},
			{Name: "Idle", Type: "trigger", TotalLines: 1, UncoveredLines: []int{2}},
		}},
	}
	got := NativePerTestRows(run)
	if a, b := fmt.Sprint(got.Suites[0].Cases[0].Coverage), fmt.Sprint(got.Suites[0].Cases[1].Coverage); a != "[{Fired trigger  1 1 [2]}]" || b != "[{Fired trigger  0 2 []}]" {
		t.Fatalf("rows a=%s b=%s", a, b)
	}
}

package cpurisk

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestEstimateUsesCapturedRowWeights(t *testing.T) {
	// 1000 Account rows: insert 707 ms and query 5 ms in the native capture.
	got := DefaultWeights().Estimate(vm.WorkCounts{DMLRows: 1000, SOQLRows: 1000})
	if got != 712 {
		t.Fatalf("estimate = %v, want 712", got)
	}
}

func TestLevelThresholds(t *testing.T) {
	cases := map[float64]string{0: "ok", 4999: "ok", 5000: "WATCH", 7999: "WATCH", 8000: "HIGH", 9999: "HIGH", 10000: "OVER", 50000: "OVER"}
	for ms, want := range cases {
		if got := Level(ms); got != want {
			t.Errorf("Level(%v) = %s, want %s", ms, got, want)
		}
	}
}

func sampleCollector() *Collector {
	c := NewCollector()
	c.Record("QuietTest", "small", vm.WorkCounts{DMLRows: 2}, nil)
	c.Record("BulkTest", "big", vm.WorkCounts{DMLRows: 15000, SOQLRows: 20000}, []vm.MethodWork{
		{Method: "BulkTest.big", Calls: 1, Work: vm.WorkCounts{DMLRows: 15000}},
		{Method: "Loader.load", Calls: 3, Work: vm.WorkCounts{DMLRows: 15000, SOQLRows: 20000}},
		{Method: "Util.tiny", Calls: 9, Work: vm.WorkCounts{LoopIterations: 10}},
	})
	c.Record("BulkTest", "medium", vm.WorkCounts{DMLRows: 800}, []vm.MethodWork{
		{Method: "Loader.load", Calls: 1, Work: vm.WorkCounts{DMLRows: 800}},
	})
	return c
}

func TestReportOrdersAndWarns(t *testing.T) {
	report := sampleCollector().Report()
	if len(report.Tests) != 3 || report.Tests[0].Method != "big" || report.Tests[2].Method != "small" {
		t.Fatalf("tests = %+v", report.Tests)
	}
	if report.Tests[0].Level != "OVER" || report.Tests[1].Level != "ok" {
		t.Fatalf("levels = %s %s", report.Tests[0].Level, report.Tests[1].Level)
	}
	// The test's own method is not repeated in the method table; a method
	// shown for several tests uses its worst test and the summed calls.
	if len(report.Methods) != 2 || report.Methods[0].Method != "Loader.load" || report.Methods[0].Calls != 4 {
		t.Fatalf("methods = %+v", report.Methods)
	}
	if report.Methods[0].Work.DMLRows != 15000 {
		t.Fatalf("worst-test work = %+v", report.Methods[0].Work)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "BulkTest.big") || !strings.Contains(report.Warnings[0], "exceeds the 10 s synchronous limit") {
		t.Fatalf("warnings = %v", report.Warnings)
	}
}

func TestWriteTextFormat(t *testing.T) {
	var out bytes.Buffer
	if err := WriteText(&out, sampleCollector().Report(), 2); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"CPU risk (advisory)", "never changes a test result", "DML row", "captured", "approximate",
		"Top tests by estimated CPU (2 of 3):", "OVER", "BulkTest.big", "Top methods", "Loader.load", "Warnings:",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "QuietTest.small") {
		t.Errorf("top 2 should omit the third test:\n%s", text)
	}
}

func TestWriteTextWithoutWarningsAndWithoutTests(t *testing.T) {
	c := NewCollector()
	var out bytes.Buffer
	if err := WriteText(&out, c.Report(), 10); err != nil || !strings.Contains(out.String(), "No tests ran.") {
		t.Fatalf("empty report: %v\n%s", err, out.String())
	}
	c.Record("A", "b", vm.WorkCounts{DMLRows: 10}, nil)
	out.Reset()
	if err := WriteText(&out, c.Report(), 10); err != nil || !strings.Contains(out.String(), "No test is near the 10 s synchronous limit") {
		t.Fatalf("quiet report: %v\n%s", err, out.String())
	}
}

func TestWriteJSONShape(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, sampleCollector().Report()); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"advisory", "weights", "weightSources", "syncLimitMs", "asyncLimitMs", "tests", "methods", "warnings"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("JSON missing %q", key)
		}
	}
	if decoded["syncLimitMs"].(float64) != 10000 || decoded["asyncLimitMs"].(float64) != 60000 {
		t.Errorf("limits = %v %v", decoded["syncLimitMs"], decoded["asyncLimitMs"])
	}
}

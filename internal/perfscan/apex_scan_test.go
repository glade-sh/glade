package perfscan

import (
	"path/filepath"
	"testing"
)

func TestAnalyzeProjectFindsApexPerformanceRisks(t *testing.T) {
	report, err := AnalyzeProject(Options{ProjectRoot: filepath.Join("testdata", "perf-project")})
	if err != nil {
		t.Fatal(err)
	}
	report.Finalize()

	assertFinding(t, report, "perf.entry.trigger")
	assertFinding(t, report, "perf.soql.loop")
	assertFinding(t, report, "perf.dml.loop")
	assertFinding(t, report, "perf.describe.repeated")
	assertFinding(t, report, "perf.async.loop")
	assertFinding(t, report, "perf.ui.auraenabled.uncached")
	assertFinding(t, report, "perf.async.batch.unfiltered-start")
	assertFinding(t, report, "perf.async.future")
	assertFinding(t, report, "perf.async.future.callout-dml")
	assertFinding(t, report, "perf.async.queueable.cycle")
	assertFinding(t, report, "perf.async.queueable.chain-depth")
	assertFinding(t, report, "perf.async.batch.execute")
	assertFinding(t, report, "perf.async.batch.finish")
	assertFinding(t, report, "perf.async.batch.execute-queueable")
	assertFinding(t, report, "perf.async.schedule.recursive")
	assertFinding(t, report, "perf.soql.unfiltered")
	assertFinding(t, report, "perf.soql.selectivity")
	assertFinding(t, report, "perf.soql.overfetch")
	assertFinding(t, report, "perf.soql.subquery-no-limit")
	assertFinding(t, report, "perf.soql.where-formula")
	assertFinding(t, report, "perf.soql.orderby-no-index")
	assertFindingWithSeverity(t, report, "perf.soql.unfiltered", SeverityMedium)

	assertNoFindingAtLine(t, report, "perf.soql.loop", 28)
	assertNoFindingAtLine(t, report, "perf.dml.loop", 38)
	assertNoFindingAtLine(t, report, "perf.dml.loop", 40)
	assertNoFindingAtLine(t, report, "perf.async.loop", 37)
	assertNoFindingAtLine(t, report, "perf.soql.unfiltered", 104)
	assertNoFindingAtLine(t, report, "perf.ui.auraenabled.uncached", 14)
	assertNoFindingAtLine(t, report, "perf.ui.auraenabled.uncached", 19)
	assertNoFindingAtLine(t, report, "perf.ui.auraenabled.uncached", 24)
	assertNoFindingAtLine(t, report, "perf.ui.auraenabled.uncached", 29)
	assertNoFindingAtLine(t, report, "perf.soql.unfiltered", 126)
}

func assertFinding(t *testing.T, report Report, id string) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID == id {
			return
		}
	}
	t.Fatalf("missing finding %s in %#v", id, report.Findings)
}

func assertNoFindingAtLine(t *testing.T, report Report, id string, line int) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID != id {
			continue
		}
		if finding.Location.Line == line {
			t.Fatalf("unexpected finding %s at line %d", id, line)
		}
	}
}

func assertFindingWithSeverity(t *testing.T, report Report, id string, severity Severity) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID != id {
			continue
		}
		if finding.Severity == severity {
			return
		}
	}
	t.Fatalf("missing finding %s with severity %s in %#v", id, severity, report.Findings)
}

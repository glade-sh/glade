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

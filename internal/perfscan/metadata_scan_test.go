package perfscan

import (
	"path/filepath"
	"testing"
)

func TestAnalyzeProjectFindsUIDeclarativePerformanceRisks(t *testing.T) {
	report, err := AnalyzeProject(Options{ProjectRoot: filepath.Join("testdata", "perf-project")})
	if err != nil {
		t.Fatal(err)
	}
	report.Finalize()

	assertFinding(t, report, "perf.ui.visualforce.action")
	assertFinding(t, report, "perf.ui.aura.server-action")
	assertFinding(t, report, "perf.ui.lwc.wire-apex")
	assertFinding(t, report, "perf.automation.flow.data-fanout")
	assertFinding(t, report, "perf.automation.workflow.active-rule")
	assertEntryPoint(t, report, EntryVisualforce)
	assertEntryPoint(t, report, EntryAura)
	assertEntryPoint(t, report, EntryLWC)
	assertEntryPoint(t, report, EntryFlow)
	assertEntryPoint(t, report, EntryWorkflow)
}

func assertEntryPoint(t *testing.T, report Report, kind EntryKind) {
	t.Helper()
	for _, entry := range report.EntryPoints {
		if entry.Kind == kind {
			return
		}
	}
	t.Fatalf("missing entry point %s in %#v", kind, report.EntryPoints)
}

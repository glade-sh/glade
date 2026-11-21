package scripts

import (
	"strings"
	"testing"
)

func TestCIEarlyGuardsWorkflow(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	job := jobs["early-guards"]
	if job == "" {
		t.Fatal("missing early guard job")
	}
	if !strings.Contains(job, "if: github.event_name == 'pull_request'") || strings.Contains(job, "\n    needs:") {
		t.Error("early guards must start independently on every pull request")
	}
	for _, name := range []string{
		"TestCIApexSkipSitesMatchResultPolicy",
		"TestCIRaceNightlySelectionRegexesSelectGoConcurrencyOnly",
		"TestCIRaceSampleSelectionRule",
		"TestCIPackageManifestConcurrencyCounts",
		"TestObjectStateRecordsWritesUseFunnel",
	} {
		if !strings.Contains(job, name) {
			t.Errorf("early job does not run %s", name)
		}
	}
	if !strings.Contains(jobs["required-ci"], "      - early-guards\n") {
		t.Error("Required CI does not depend on the early guard result")
	}
}

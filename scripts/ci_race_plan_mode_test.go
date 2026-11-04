package scripts

import (
	"os"
	"strings"
	"testing"
)

// Full-mode inputs must not delay or fail the concurrency-only push plan.
func TestCIRacePlanPreparationOnlyInFullMode(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/race.yml")
	if err != nil {
		t.Fatal(err)
	}
	plan := workflowJobBlocksText(string(data))["plan"]
	full := "steps.classify.outputs.mode == 'full'"
	consumer := "contains(fromJSON(steps.classify.outputs.early-packages), './internal/gladecli')"
	for _, marker := range []string{
		"      - uses: actions/setup-node@", "      - name: Resolve race npm cache path", "      - name: Restore race npm cache", "      - name: Install shared LWC toolchain", "      - name: Save race npm cache",
	} {
		step := workflowStepBlockText(plan, marker)
		if !strings.Contains(step, "if: ${{ "+full+" && "+consumer+" }}") {
			t.Errorf("%s must require full mode and its consumer", marker)
		}
	}
	for _, tc := range []struct{ marker, condition string }{
		{"Find validated LWC timing evidence", full},
		{"Download LWC timing evidence", full + " && steps.lwc-history.outputs.run_id != ''"},
		{"Validate LWC duration history", full + " && steps.lwc-history.outputs.run_id != '' && steps.lwc-timing-download.outcome == 'success'"},
		{"Stage shared LWC duration history", full},
		{"Upload shared LWC duration history", full},
	} {
		step := workflowStepBlockText(plan, "      - name: "+tc.marker)
		if !strings.Contains(step, "if: "+tc.condition+"\n") {
			t.Errorf("%s must use condition %s", tc.marker, tc.condition)
		}
	}
}

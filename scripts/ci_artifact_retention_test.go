package scripts

import (
	"strings"
	"testing"
)

func TestCIArtifactRetention(t *testing.T) {
	count := 0
	for jobName, job := range readPRLaneJobs(t) {
		for _, step := range job.Steps {
			if !strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
				continue
			}
			count++
			name := step.With["name"]
			want := "7"
			if name == "ci-tree" || strings.Contains(name, "-duration-history-") {
				want = "90"
			}
			if got := step.With["retention-days"]; got != want {
				t.Errorf("%s: artifact %s retention = %q, want %s", jobName, name, got, want)
			}
		}
	}
	if count == 0 {
		t.Fatal("no CI artifact uploads found")
	}
}

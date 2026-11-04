package scripts

import (
	"strings"
	"testing"
)

func TestCIScorecardSkipsTagPushes(t *testing.T) {
	job := workflowJobBlocks(t, readWorkflowFile(t, "security.yml"))["scorecard"]
	if !strings.Contains(job, "if: ${{ !startsWith(github.ref, 'refs/tags/') }}") {
		t.Fatal("Scorecard must skip tag refs while remaining available for branch pushes, pull requests, and schedules")
	}
}

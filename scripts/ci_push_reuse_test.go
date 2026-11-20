package scripts

import (
	"strings"
	"testing"
)

func TestCIWorkflowMainPushReceiptGate(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	gate := jobs["tree-reuse-gate"]
	for _, marker := range []string{"ci-prepare-tree-reuse.sh", "github.event.before", "github.sha", "continue-on-error: true"} {
		if !strings.Contains(gate, marker) {
			t.Errorf("tree reuse gate missing %q", marker)
		}
	}
	for _, job := range []string{"apextest", "vm", "gladecli", "sema", "lwc-vf", "site", "test", "workflow-lint"} {
		if !strings.Contains(jobs[job], "needs.tree-reuse-gate.outputs.reuse") {
			t.Errorf("%s does not depend on the reuse decision", job)
		}
	}
	if strings.Contains(jobs["leak-scan"], "needs.tree-reuse-gate.outputs.reuse") {
		t.Error("push-range leak scan may not be skipped")
	}
	for _, marker := range []string{"name: Reused tree CI", "needs.tree-reuse-gate.outputs.reuse"} {
		if !strings.Contains(jobs["reused-tree-ci"], marker) {
			t.Errorf("reuse marker missing %q", marker)
		}
	}
	for _, marker := range []string{"needs.tree-reuse-gate.outputs.reuse", "reuse-history", "actions/cache/save@"} {
		if !strings.Contains(jobs["history"], marker) {
			t.Errorf("history maintenance missing %q", marker)
		}
	}
}

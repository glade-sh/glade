package scripts

import (
	"strings"
	"testing"
)

func TestCICodeQLSecurityOwnershipAndReleaseGate(t *testing.T) {
	_, ci := readCIWorkflow(t)
	security := workflowJobBlocks(t, readWorkflowFile(t, "security.yml"))
	release := workflowJobBlocks(t, readWorkflowFile(t, "release.yml"))
	if _, ok := ci["codeql"]; ok || strings.Contains(ci["required-ci"], "      - codeql\n") {
		t.Error("CI still owns CodeQL")
	}
	for _, marker := range []string{"profile: fast", "github.event_name == 'pull_request'", "github.ref_type != 'tag'"} {
		if !strings.Contains(security["codeql-fast"], marker) {
			t.Errorf("fast Security route missing %s", marker)
		}
	}
	for _, marker := range []string{"profile: full", "github.ref_type == 'tag'", "github.event_name == 'schedule'"} {
		if !strings.Contains(security["codeql"], marker) {
			t.Errorf("full Security route missing %s", marker)
		}
	}
	for _, marker := range []string{"profile: full", "refs/tags/"} {
		if !strings.Contains(release["codeql-release"], marker) {
			t.Errorf("tag release scan missing %s", marker)
		}
	}
	if !strings.Contains(release["prepare"], "codeql-release") || !strings.Contains(release["shared-payload"], "needs.codeql-release.result") {
		t.Error("release paths do not require completed scan")
	}
}

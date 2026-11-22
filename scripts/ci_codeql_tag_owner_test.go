package scripts

import (
	"strings"
	"testing"
)

func TestCICodeQLSingleTagOwner(t *testing.T) {
	security := workflowJobBlocks(t, readWorkflowFile(t, "security.yml"))
	release := workflowJobBlocks(t, readWorkflowFile(t, "release.yml"))
	full := security["codeql"]
	if strings.Contains(full, "github.ref_type == 'tag'") || !strings.Contains(full, "github.ref_type != 'tag'") {
		t.Error("Security full scan must exclude tags, including manual tag dispatches")
	}
	if !strings.Contains(release["codeql-release"], "startsWith(github.ref, 'refs/tags/')") || !strings.Contains(release["codeql-release"], "profile: full") {
		t.Error("release must own the full tag scan")
	}
	if !strings.Contains(release["prepare"], "needs.codeql-release.result == 'success'") {
		t.Error("release preparation must require the scan")
	}
}

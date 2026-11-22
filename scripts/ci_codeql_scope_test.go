package scripts

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCICodeQLChangeScope(t *testing.T) {
	for _, tc := range []struct{ name, paths, want string }{
		{"docs", "docs/guide.md\x00README.md\x00", "false"},
		{"nested markdown", "internal/vm/README.md\x00", "false"},
		{"go source", "internal/vm/example.go\x00", "true"},
		{"docs source", "docs/example.go\x00", "true"},
		{"fixture", "testdata/README.md\x00", "true"},
		{"mixed", "README.md\x00go.mod\x00", "true"},
		{"empty", "", "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("python3", "ci-codeql-scope.py")
			cmd.Stdin = strings.NewReader(tc.paths)
			out, err := cmd.CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != tc.want {
				t.Fatalf("got %q, %v; want %s", out, err, tc.want)
			}
		})
	}
	jobs := workflowJobBlocks(t, readWorkflowFile(t, "security.yml"))
	if !strings.Contains(jobs["codeql-fast"], "needs.codeql-scope.outputs.run == 'true'") || !strings.Contains(jobs["codeql-scope"], "git diff --no-renames") {
		t.Error("fast scan must use the complete PR change scope")
	}
}

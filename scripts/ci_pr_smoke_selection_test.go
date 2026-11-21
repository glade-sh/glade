package scripts

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCIPRSmokeSelection(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"scripts/release-build.sh", "true"},
		{"scripts/smoke-runtime.sh", "true"},
		{"scripts/sync-update-installer.sh", "true"},
		{".github/workflows/release.yml", "true"},
		{"site/index.md", "true"},
		{"Dockerfile", "true"},
		{"internal/vm/vm.go", "false"},
		{"docs/guide.md", "false"},
		{"scripts/ci-go-test.sh", "false"},
		{"scripts/release-notes.sh\ninternal/vm/vm.go", "true"},
	} {
		t.Run(strings.ReplaceAll(tc.path, "/", "_"), func(t *testing.T) {
			cmd := exec.Command("python3", "ci-pr-smoke-select.py")
			cmd.Stdin = strings.NewReader(strings.ReplaceAll(tc.path, "\n", "\x00") + "\x00")
			out, err := cmd.CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != tc.want {
				t.Fatalf("selection %q: got %q, err %v; want %s", tc.path, out, err, tc.want)
			}
		})
	}
}

func TestCIPRSmokeWorkflow(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, required := range []string{
		"ci-pr-smoke-select.py", "needs.pr-select.outputs.smoke", "needs.pr-select.result", "--name-only -z",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("missing smoke planning contract %q", required)
		}
	}
}

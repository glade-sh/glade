package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIPrepareTreeReuse(t *testing.T) {
	script, err := os.ReadFile("ci-prepare-tree-reuse.sh")
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	const sha = "1111111111111111111111111111111111111111"
	const before = "2222222222222222222222222222222222222222"
	for _, tc := range []struct {
		name string
		ok   bool
	}{
		{"complete", true}, {"wrong before", false}, {"unverified", false},
		{"missing attempt", false}, {"missing artifact", false}, {"invalid artifact", false},
		{"incomplete artifact", false},
		{"missing inventory", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			for _, dir := range []string{"scripts", ".github/workflows", "bin"} {
				if err := os.MkdirAll(filepath.Join(d, dir), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(d, "scripts/ci-prepare-tree-reuse.sh"), script, 0o700); err != nil {
				t.Fatal(err)
			}
			if tc.name == "missing inventory" {
				workflow = []byte("jobs:\n  history:\n    steps: []\n")
			}
			if err := os.WriteFile(filepath.Join(d, ".github/workflows/ci.yml"), workflow, 0o600); err != nil {
				t.Fatal(err)
			}
			verify := `#!/usr/bin/env bash
[[ "$SCENARIO" != unverified ]] || exit 1
if [[ "$SCENARIO" == 'missing attempt' ]]; then printf '{"run_id":101}'; else printf '{"run_id":101,"run_attempt":1}'; fi
`
			gh := `#!/usr/bin/env bash
case "$*" in
  *"/commits/` + sha + `"*) if [[ "$SCENARIO" == 'wrong before' ]]; then printf '{"parents":[{"sha":"ffffffffffffffffffffffffffffffffffffffff"}]}'; else printf '{"parents":[{"sha":"` + before + `"}]}'; fi ;;
  *"run download 101"*)
    [[ "$SCENARIO" != 'missing artifact' ]] || exit 1
    while [[ $# -gt 0 ]]; do case "$1" in -n) name="$2"; shift ;; -D) dest="$2"; shift ;; esac; shift; done
    mkdir -p "$dest"; stem="${name%-101-1}"
    if [[ "$SCENARIO" == 'invalid artifact' ]]; then printf 'bad' > "$dest/$stem.json"; elif [[ "$SCENARIO" == 'incomplete artifact' ]]; then printf '{"complete":false,"tests":[]}' > "$dest/$stem.json"; else printf '{"complete":true,"tests":[{"name":"TestExample","durationMillis":1}]}' > "$dest/$stem.json"; fi ;;
  *) exit 1 ;;
esac
`
			for path, content := range map[string]string{"scripts/ci-verify-pr-tree.sh": verify, "bin/gh": gh} {
				if err := os.WriteFile(filepath.Join(d, path), []byte(content), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			outDir := filepath.Join(d, "history")
			cmd := exec.Command("bash", filepath.Join(d, "scripts/ci-prepare-tree-reuse.sh"), "glade-sh/glade", sha, before, outDir)
			cmd.Env = append(os.Environ(), "PATH="+filepath.Join(d, "bin")+":"+os.Getenv("PATH"), "SCENARIO="+tc.name)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v want success=%v: %s", err, tc.ok, out)
			}
		})
	}
}

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

package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
		{"missing history declaration", false},
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
			tcWorkflow := workflow
			if tc.name == "missing inventory" {
				tcWorkflow = []byte("jobs:\n  history:\n    steps: []\n")
			}
			if tc.name == "missing history declaration" {
				tcWorkflow = []byte(strings.Replace(string(tcWorkflow), "name: gladecli-duration-history-${{ github.run_id }}-${{ github.run_attempt }}", "name: incomplete-evidence", 1))
			}
			if err := os.WriteFile(filepath.Join(d, ".github/workflows/ci.yml"), tcWorkflow, 0o600); err != nil {
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
			if tc.ok {
				files, err := filepath.Glob(filepath.Join(outDir, "*-duration-history", "*-duration-history.json"))
				if err != nil || len(files) == 0 {
					t.Fatalf("no transferred history: files=%v err=%v", files, err)
				}
				for _, path := range files {
					got, err := os.ReadFile(path)
					if err != nil || string(got) != `{"complete":true,"tests":[{"name":"TestExample","durationMillis":1}]}` {
						t.Fatalf("history bytes changed: %s: %q, %v", path, got, err)
					}
				}
			}
		})
	}
}

func TestCIRequiredCIPushReuseRequiresMaintenance(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Needs yaml.Node `yaml:"needs"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["required-ci"]
	var needs []string
	if err := job.Needs.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	check := func(results map[string]map[string]string, reuse string, want bool) {
		t.Helper()
		encoded, err := json.Marshal(results)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "-c", job.Steps[0].Run)
		cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+string(encoded), "EVENT_NAME=push", "HEAD_REF=", "TREE_REUSE="+reuse)
		out, err := cmd.CombinedOutput()
		if (err == nil) != want {
			t.Fatalf("reuse=%s results=%s err=%v want=%v: %s", reuse, encoded, err, want, out)
		}
	}
	results := map[string]map[string]string{}
	for _, name := range needs {
		results[name] = map[string]string{"result": "skipped"}
	}
	for _, name := range []string{"tree-reuse-gate", "reused-tree-ci", "leak-scan", "history", "test"} {
		results[name]["result"] = "success"
	}
	check(results, "true", true)
	for _, name := range []string{"tree-reuse-gate", "reused-tree-ci", "leak-scan", "history", "test"} {
		results[name]["result"] = "failure"
		check(results, "true", false)
		results[name]["result"] = "success"
	}
	delete(results, "history")
	check(results, "true", false)
	results["history"] = map[string]string{"result": "success"}
	check(results, "false", false)
	for _, name := range needs {
		results[name]["result"] = "success"
	}
	results["pr-tests"]["result"], results["pr-guards"]["result"], results["reused-tree-ci"]["result"] = "skipped", "skipped", "skipped"
	results["tree-reuse-gate"]["result"] = "failure"
	check(results, "false", true)
	results["site"]["result"] = "skipped"
	check(results, "false", false)
}

func TestCIWorkflowMainPushReceiptGate(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	gate := jobs["tree-reuse-gate"]
	for _, marker := range []string{"ci-prepare-tree-reuse.sh", "github.event.before", "github.sha", "continue-on-error: true"} {
		if !strings.Contains(gate, marker) {
			t.Errorf("tree reuse gate missing %q", marker)
		}
	}
	for _, job := range []string{"history-input", "test-binaries", "test-binaries-apextest", "site", "test", "workflow-lint"} {
		if !strings.Contains(jobs[job], "needs.tree-reuse-gate.outputs.reuse") {
			t.Errorf("%s does not depend on the reuse decision", job)
		}
	}
	for job, dependency := range map[string]string{"apextest": "history-input", "vm": "history-input", "gladecli": "history-input", "sema": "test-binaries", "lwc-vf": "test-binaries"} {
		if !strings.Contains(jobs[job], "needs: "+dependency) && !strings.Contains(jobs[job], dependency+",") && !strings.Contains(jobs[job], dependency+"]") {
			t.Errorf("%s does not depend on gated prerequisite %s", job, dependency)
		}
		if !strings.Contains(jobs[job], "!cancelled()") || !strings.Contains(jobs[job], "needs."+dependency+".result == 'success'") {
			t.Errorf("%s cannot recover from a failed gate after its prerequisite succeeds", job)
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
	for _, marker := range []string{"steps.reuse-history.outcome", "reuse-history", "actions/cache/save@"} {
		if !strings.Contains(jobs["history"], marker) {
			t.Errorf("history maintenance missing %q", marker)
		}
	}
}

func TestCITreeReuseDecisionDefaultsToFullRun(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				ID  string `yaml:"id"`
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	var script string
	for _, step := range workflow.Jobs["tree-reuse-gate"].Steps {
		if step.ID == "decision" {
			script = step.Run
		}
	}
	if script == "" {
		t.Fatal("missing decision step")
	}
	for _, tc := range []struct{ event, prepare, carry, want string }{
		{"push", "success", "success", "true"},
		{"push", "failure", "skipped", "false"},
		{"push", "success", "failure", "false"},
		{"push", "", "", "false"},
		{"pull_request", "success", "success", "false"},
	} {
		output := filepath.Join(t.TempDir(), "output")
		cmd := exec.Command("bash", "-c", script)
		cmd.Env = append(os.Environ(), "EVENT_NAME="+tc.event, "PREPARE_OUTCOME="+tc.prepare, "CARRY_OUTCOME="+tc.carry, "GITHUB_OUTPUT="+output)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("decision failed: %v: %s", err, out)
		}
		got, err := os.ReadFile(output)
		if err != nil || string(got) != "reuse="+tc.want+"\n" {
			t.Fatalf("%+v: output=%q err=%v", tc, got, err)
		}
	}
}

func TestCIReuseCoversFullValidationJobs(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Needs yaml.Node `yaml:"needs"`
			If    string    `yaml:"if"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	dependencies := func(name string) []string {
		t.Helper()
		node := workflow.Jobs[name].Needs
		if node.Kind == 0 {
			return nil
		}
		if node.Kind == yaml.ScalarNode {
			return []string{node.Value}
		}
		var names []string
		if err := node.Decode(&names); err != nil {
			t.Fatal(err)
		}
		return names
	}
	var guarded func(string, map[string]bool) bool
	guarded = func(name string, seen map[string]bool) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		job := workflow.Jobs[name]
		for _, dependency := range dependencies(name) {
			if dependency == "tree-reuse-gate" && strings.Contains(job.If, "needs.tree-reuse-gate.outputs.reuse != 'true'") && strings.Contains(job.If, "needs.tree-reuse-gate.result != 'success'") {
				return true
			}
			if strings.Contains(job.If, "!cancelled()") && strings.Contains(job.If, "needs."+dependency+".result == 'success'") && guarded(dependency, seen) {
				return true
			}
		}
		return false
	}
	for _, name := range dependencies("required-ci") {
		switch name {
		case "tree-reuse-gate", "reused-tree-ci", "leak-scan", "history", "test", "pr-tests", "pr-guards":
			continue
		}
		if !guarded(name, map[string]bool{}) {
			t.Errorf("required validation job %s is not gated directly or through a successful prerequisite", name)
		}
	}
}

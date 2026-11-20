package scripts

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCIPRRaceSelection(t *testing.T) {
	cmd := exec.Command("python3", "ci_pr_race_selection_test.py")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("race selection tests: %v\n%s", err, output)
	}
}

func TestCIPRRaceWorkflow(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/race-pr.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On   map[string]any `yaml:"on"`
		Jobs map[string]struct {
			If    string `yaml:"if"`
			Needs string `yaml:"needs"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	if _, ok := workflow.On["pull_request"]; !ok {
		t.Fatal("race pilot must run on pull requests")
	}
	var commands string
	for _, step := range workflow.Jobs["select"].Steps {
		commands += step.Run + "\n"
	}
	for _, required := range []string{"ci-pr-select.py", "ci-pr-race-select.py", "--docs-only"} {
		if !strings.Contains(commands, required) {
			t.Errorf("selection job does not invoke %s", required)
		}
	}
	job := workflow.Jobs["race"]
	if job.Needs != "select" || !strings.Contains(job.If, "needs.select.outputs.packages") {
		t.Fatal("race jobs must depend on the selected package list")
	}
}

package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The org runs 60 jobs at once. Keep concurrent jobs below that limit with
// headroom so one run never queues behind itself.
const ciPushJobBudget = 58

func TestCIWorkflowJobCountsStayInsideRunnerLimit(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			If       string    `yaml:"if"`
			Needs    yaml.Node `yaml:"needs"`
			Strategy struct {
				Matrix map[string]any `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{"push": 0, "pull_request": 0}
	for name, job := range workflow.Jobs {
		// The gate precedes the full jobs, and the reused marker is mutually
		// exclusive with those jobs. Neither adds to the full-run peak.
		if name == "tree-reuse-gate" {
			continue
		}
		if name == "reused-tree-ci" {
			if !strings.Contains(job.If, "needs.tree-reuse-gate.outputs.reuse == 'true'") {
				t.Fatal("reuse marker is not conditional on the gate")
			}
			continue
		}
		// Workflow lint and the leak scan still run serially, with the gate
		// as an additional prerequisite for workflow lint.
		var dependencies []string
		if job.Needs.Kind == yaml.ScalarNode {
			dependencies = []string{job.Needs.Value}
		} else if job.Needs.Kind != 0 {
			if err := job.Needs.Decode(&dependencies); err != nil {
				t.Fatal(err)
			}
		}
		if name == "workflow-lint" && len(job.Strategy.Matrix) == 0 && workflow.Jobs["leak-scan"].If == "" && containsDependency(dependencies, "leak-scan") {
			continue
		}
		width := 1
		if include, ok := job.Strategy.Matrix["include"].([]any); ok {
			width = len(include)
		} else if len(job.Strategy.Matrix) > 0 {
			width = 0
			for _, values := range job.Strategy.Matrix {
				if list, ok := values.([]any); ok {
					if width == 0 {
						width = 1
					}
					width *= len(list)
				}
			}
		}
		if width == 0 {
			t.Fatalf("job %s has an unreadable matrix", name)
		}
		onlyPR := strings.Contains(job.If, "github.event_name == 'pull_request'") && !strings.Contains(job.If, "!=")
		onlySchedule := strings.Contains(job.If, "github.event_name == 'schedule'")
		notPR := strings.Contains(job.If, "github.event_name != 'pull_request'")
		if !onlyPR && !onlySchedule {
			if !strings.Contains(job.If, "github.event_name == 'pull_request'") || notPR {
				counts["push"] += width
			}
		}
		if !notPR && !onlySchedule {
			counts["pull_request"] += width
		}
	}
	if counts["push"] > ciPushJobBudget {
		t.Errorf("push run schedules %d jobs, budget %d (org limit 60)", counts["push"], ciPushJobBudget)
	}
	if counts["pull_request"] > ciPushJobBudget {
		t.Errorf("pull_request run schedules %d jobs, budget %d (org limit 60)", counts["pull_request"], ciPushJobBudget)
	}
	t.Logf("push=%d pull_request=%d", counts["push"], counts["pull_request"])
}

func containsDependency(dependencies []string, name string) bool {
	for _, dependency := range dependencies {
		if dependency == name {
			return true
		}
	}
	return false
}

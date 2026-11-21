package scripts

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCIRequiredPRPlan(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	job := jobs["required-ci"]
	if !strings.Contains(job, "    if: ${{ !cancelled() }}") {
		t.Error("aggregate must stop on workflow cancellation")
	}
	script := requiredAggregateShell(t, job)
	results := `{"pr-select":{"result":"success"},"pr-tests":{"result":"success"},"early-guards":{"result":"success"},"pr-guards":{"result":"success"},"workflow-lint":{"result":"success"},"site":{"result":"success"},"vet":{"result":"success"},"leak-scan":{"result":"success"}}`
	for _, tc := range []struct {
		name, plan, results string
		success             bool
	}{
		{"empty selection sentinel", `{"include":[{"shard":-1,"packages":[],"pool":[],"browser":false}]}`, results, true},
		{"selected worker", `{"include":[{"shard":12,"packages":["x"],"pool":["x"],"browser":false}]}`, results, true},
		{"missing plan", ``, results, false},
		{"empty matrix", `{"include":[]}`, results, false},
		{"duplicate worker", `{"include":[{"shard":1,"packages":["x"],"pool":[],"browser":false},{"shard":1,"packages":["x"],"pool":[],"browser":false}]}`, results, false},
		{"empty real worker", `{"include":[{"shard":1,"packages":[],"pool":[],"browser":false}]}`, results, false},
		{"cancelled matrix", `{"include":[{"shard":-1,"packages":[],"pool":[],"browser":false}]}`, strings.Replace(results, `"pr-tests":{"result":"success"}`, `"pr-tests":{"result":"cancelled"}`, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+tc.results, "EVENT_NAME=pull_request", "HEAD_REF=", "SMOKE_REQUIRED=false", "PR_PLAN="+tc.plan)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.success {
				t.Fatalf("result %v output %s", err, out)
			}
		})
	}
}

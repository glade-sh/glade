package scripts

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCIRequiredPRPlanCoversSelectedRoutes(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	script := requiredAggregateShell(t, jobs["required-ci"])
	results := `{"pr-select":{"result":"success"},"pr-tests":{"result":"success"},"early-guards":{"result":"success"},"pr-guards":{"result":"success"},"workflow-lint":{"result":"success"},"site":{"result":"success"},"vet":{"result":"success"},"leak-scan":{"result":"success"}}`
	const apex = "github.com/glade-sh/glade/internal/apextest"
	planner := exec.Command("python3", "ci-pr-plan.py")
	planner.Stdin = strings.NewReader(`["` + apex + `"]`)
	valid, err := planner.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, selected, plan string
		want                 bool
	}{
		{"valid", `["` + apex + `"]`, string(valid), true},
		{"misrouted", `["` + apex + `"]`, `{"include":[{"shard":44,"packages":["` + apex + `"],"pool":[],"browser":false}]}`, false},
		{"omitted", `["` + apex + `"]`, `{"include":[{"shard":-1,"packages":[],"pool":[],"browser":false}]}`, false},
		{"missing selection", ``, string(valid), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", script)
			cmd.Dir = ".."
			cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+results, "EVENT_NAME=pull_request", "HEAD_REF=", "SMOKE_REQUIRED=false", "PR_BROWSER=false", "PR_PLAN="+tc.plan, "PR_SELECTION="+tc.selected)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.want {
				t.Fatalf("result %v, want success=%v: %s", err, tc.want, out)
			}
		})
	}
}

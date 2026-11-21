package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIRequiredPRPlanCoversSelectedRoutes(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	script := requiredAggregateShell(t, jobs["required-ci"])
	selector := jobs["pr-select"]
	validation := strings.Index(selector, "python3 scripts/ci-pr-validate-plan.py")
	emit := strings.Index(selector, `echo "matrix=$matrix"`)
	if validation < 0 || emit < 0 || validation > emit {
		t.Fatal("selector must validate the selected packages and matrix before emitting it")
	}
	results := `{"pr-select":{"result":"success"},"pr-tests":{"result":"success"},"early-guards":{"result":"success"},"pr-guards":{"result":"success"},"workflow-lint":{"result":"success"},"site":{"result":"success"},"vet":{"result":"success"},"leak-scan":{"result":"success"}}`
	const apex = "github.com/glade-sh/glade/internal/apextest"
	planner := exec.Command("python3", "ci-pr-plan.py")
	planner.Stdin = strings.NewReader(`["` + apex + `"]`)
	valid, err := planner.Output()
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Include []json.RawMessage `json:"include"`
	}
	if err := json.Unmarshal(valid, &decoded); err != nil || len(decoded.Include) < 2 {
		t.Fatalf("expected multi-shard Apex plan: %v", err)
	}
	decoded.Include = decoded.Include[1:]
	partial, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, selected, plan string
		want                 bool
	}{
		{"valid", `["` + apex + `"]`, string(valid), true},
		{"misrouted", `["` + apex + `"]`, `{"include":[{"shard":44,"packages":["` + apex + `"],"pool":[],"browser":false}]}`, false},
		{"missing shard", `["` + apex + `"]`, string(partial), false},
		{"pool misroute", `["example/pkg"]`, `{"include":[{"shard":44,"packages":["example/pkg"],"pool":["example/pkg"],"browser":false}]}`, false},
		{"omitted", `["` + apex + `"]`, `{"include":[{"shard":-1,"packages":[],"pool":[],"browser":false}]}`, false},
		{"missing selection", ``, string(valid), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			validator := exec.Command("python3", "ci-pr-validate-plan.py", tc.selected, tc.plan)
			if err := validator.Run(); (err == nil) != tc.want {
				t.Fatalf("validator result %v, want success=%v", err, tc.want)
			}
			cmd := exec.Command("bash", "-c", script)
			cmd.Dir = ".."
			cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+results, "EVENT_NAME=pull_request", "HEAD_REF=", "SMOKE_REQUIRED=false", "PR_BROWSER=false", "PR_PLAN="+tc.plan, "PR_SELECTION="+tc.selected, "PR_VALIDATED_PLAN="+string(valid))
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.want {
				t.Fatalf("result %v, want success=%v: %s", err, tc.want, out)
			}
		})
	}
}

func TestCIValidatePRPlanSelectedRoutes(t *testing.T) {
	for _, selected := range []string{
		`[]`,
		`["github.com/glade-sh/glade/internal/gladecli","github.com/glade-sh/glade/internal/server","github.com/glade-sh/glade/internal/storage","github.com/glade-sh/glade/internal/repoguard","github.com/glade-sh/glade/scripts"]`,
	} {
		planner := exec.Command("python3", "ci-pr-plan.py")
		planner.Stdin = strings.NewReader(selected)
		plan, err := planner.CombinedOutput()
		if err != nil {
			t.Fatalf("plan %s: %v: %s", selected, err, plan)
		}
		validator := exec.Command("python3", "ci-pr-validate-plan.py", selected, string(plan))
		if out, err := validator.CombinedOutput(); err != nil {
			t.Fatalf("validate %s: %v: %s", selected, err, out)
		}
	}
}

func TestCIValidatePRPlanRequiresBrowserEvidence(t *testing.T) {
	const selected = `["github.com/glade-sh/glade/internal/server"]`
	planner := exec.Command("python3", "ci-pr-plan.py")
	planner.Stdin = strings.NewReader(selected)
	plan, err := planner.Output()
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(plan), `"browser":true`, `"browser":false`, 1)
	if bad == string(plan) {
		t.Fatal("server plan lacks browser worker")
	}
	cmd := exec.Command("python3", "ci-pr-validate-plan.py", selected, bad)
	if err := cmd.Run(); err == nil {
		t.Fatal("validator accepted a browser worker without browser evidence")
	}
}

func TestCIValidatePRPlanTracksWorkflowWorkerRoutes(t *testing.T) {
	workflow, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, selected, old, replacement string }{
		{"service", `["github.com/glade-sh/glade/internal/server"]`, `SHARD == 44`, `SHARD == 43`},
		{"cli", `["github.com/glade-sh/glade/internal/gladecli"]`, `"$SHARD" == 22`, `"$SHARD" == 21`},
		{"pool", `["github.com/glade-sh/glade/internal/storage"]`, `SHARD >= 12 && SHARD < 24`, `SHARD >= 24 && SHARD < 25`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.Replace(string(workflow), tc.old, tc.replacement, 1)
			if mutated == string(workflow) {
				t.Fatal("workflow route fixture did not change")
			}
			path := filepath.Join(t.TempDir(), "ci.yml")
			if err := os.WriteFile(path, []byte(mutated), 0600); err != nil {
				t.Fatal(err)
			}
			planner := exec.Command("python3", "ci-pr-plan.py")
			planner.Stdin = strings.NewReader(tc.selected)
			plan, err := planner.Output()
			if err != nil {
				t.Fatal(err)
			}
			if err := exec.Command("python3", "ci-pr-validate-plan.py", tc.selected, string(plan), path).Run(); err == nil {
				t.Fatal("validator accepted a plan for a different workflow route")
			}
		})
	}
}

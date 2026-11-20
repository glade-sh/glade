package scripts

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestCIPRPlanKeepsSelectedRoutesAndSkipsEmptyWorkers(t *testing.T) {
	cmd := exec.Command("python3", "ci-pr-plan.py", "--manifest", "ci-package-lanes.json")
	cmd.Stdin = strings.NewReader(`["github.com/glade-sh/glade/internal/apextest","github.com/glade-sh/glade/internal/visualforce"]`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	var plan struct {
		Include []struct {
			Shard    int      `json:"shard"`
			Packages []string `json:"packages"`
			Browser  bool     `json:"browser"`
		} `json:"include"`
	}
	if err := json.Unmarshal(out, &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Include) != 20 {
		t.Fatalf("got %d workers, want 12 Apex and 8 Visualforce: %s", len(plan.Include), out)
	}
	for _, worker := range plan.Include {
		if worker.Shard < 12 && (worker.Browser || len(worker.Packages) != 1 || !strings.HasSuffix(worker.Packages[0], "/apextest")) {
			t.Errorf("Apex worker: %+v", worker)
		}
		if worker.Shard >= 12 && (worker.Shard < 36 || worker.Shard >= 44 || !worker.Browser || len(worker.Packages) != 1 || !strings.HasSuffix(worker.Packages[0], "/visualforce")) {
			t.Errorf("Visualforce worker: %+v", worker)
		}
	}
}

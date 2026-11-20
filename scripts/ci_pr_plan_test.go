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

func TestCIPRPlanEmptyAndPoolAssignment(t *testing.T) {
	for _, input := range []string{`[]`, `["github.com/glade-sh/glade/internal/vm","github.com/glade-sh/glade/internal/lwcbrowser","github.com/glade-sh/glade/internal/storage"]`} {
		cmd := exec.Command("python3", "ci-pr-plan.py")
		cmd.Stdin = strings.NewReader(input)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("plan %s: %v: %s", input, err, out)
		}
		var p struct {
			Include []struct {
				Shard    int      `json:"shard"`
				Packages []string `json:"packages"`
				Browser  bool     `json:"browser"`
			} `json:"include"`
		}
		if err := json.Unmarshal(out, &p); err != nil {
			t.Fatal(err)
		}
		if input == `[]` {
			if len(p.Include) != 1 || p.Include[0].Shard != -1 || p.Include[0].Browser {
				t.Fatalf("empty plan: %s", out)
			}
			continue
		}
		seen := map[string]int{}
		for _, w := range p.Include {
			if w.Shard < 12 || w.Shard >= 24 {
				t.Errorf("unexpected worker %+v", w)
			}
			browser := false
			for _, pkg := range w.Packages {
				seen[pkg]++
				if strings.HasSuffix(pkg, "/lwcbrowser") {
					browser = true
				}
			}
			if w.Browser != browser {
				t.Errorf("browser worker %+v", w)
			}
		}
		if seen["github.com/glade-sh/glade/internal/vm"] != 4 || seen["github.com/glade-sh/glade/internal/storage"] != 1 || seen["github.com/glade-sh/glade/internal/lwcbrowser"] != 1 {
			t.Fatalf("coverage: %v", seen)
		}
	}
}

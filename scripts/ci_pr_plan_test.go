package scripts

import (
	"encoding/json"
	"fmt"
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

func TestCIPRPlanPreservesPoolAssignments(t *testing.T) {
	selected := `["github.com/glade-sh/glade/internal/vm","github.com/glade-sh/glade/internal/gladecli","github.com/glade-sh/glade/internal/sema","github.com/glade-sh/glade/internal/storage","github.com/glade-sh/glade/internal/lwcbrowser","github.com/glade-sh/glade/internal/server"]`
	cmd := exec.Command("python3", "ci-pr-plan.py")
	cmd.Stdin = strings.NewReader(selected)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plan: %v: %s", err, out)
	}
	var p struct {
		Include []struct {
			Shard int      `json:"shard"`
			Pool  []string `json:"pool"`
		} `json:"include"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	got := map[int][]string{}
	for _, w := range p.Include {
		got[w.Shard] = w.Pool
	}
	for i := 0; i < 12; i++ {
		assign := exec.Command("python3", "ci-pr-assign.py", "--worker", fmt.Sprint(i), "--workers", "12")
		assign.Stdin = strings.NewReader(selected)
		out, err := assign.CombinedOutput()
		if err != nil {
			t.Fatalf("assignment %d: %v: %s", i, err, out)
		}
		if strings.Join(got[i+12], "\n") != strings.TrimSpace(string(out)) {
			t.Errorf("worker %d: got %v, want %s", i+12, got[i+12], out)
		}
	}
}

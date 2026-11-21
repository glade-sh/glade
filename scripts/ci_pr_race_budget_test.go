package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIPRRaceBudgets(t *testing.T) {
	root := t.TempDir()
	budget := filepath.Join(root, "budget.json")
	fixture := `{"default":{"testMinutes":9,"jobMinutes":11},"samples":[{"package":"example/slow","testSeconds":540,"multiplier":1.75,"setupSeconds":200,"marginSeconds":60}]}`
	if err := os.WriteFile(budget, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(input string) ([]byte, error) {
		cmd := exec.Command("python3", "ci-pr-race-budgets.py", budget)
		cmd.Stdin = strings.NewReader(input)
		return cmd.CombinedOutput()
	}
	out, err := run(`["example/fast","example/slow"]`)
	if err != nil {
		t.Fatalf("budget planner: %v: %s", err, out)
	}
	var matrix struct {
		Include []struct {
			Package     string `json:"package"`
			TestMinutes int    `json:"test_minutes"`
			JobMinutes  int    `json:"job_minutes"`
		} `json:"include"`
	}
	if err := json.Unmarshal(out, &matrix); err != nil {
		t.Fatal(err)
	}
	if len(matrix.Include) != 2 || matrix.Include[0].TestMinutes != 9 || matrix.Include[0].JobMinutes != 11 || matrix.Include[1].TestMinutes != 16 || matrix.Include[1].JobMinutes != 21 {
		t.Fatalf("unexpected budgets: %s", out)
	}
	if out, err := run(`[]`); err != nil || strings.TrimSpace(string(out)) != "[]" {
		t.Fatalf("empty selection: %v: %s", err, out)
	}
	if out, err := run(`["example/slow","example/slow"]`); err == nil {
		t.Fatalf("duplicate selection accepted: %s", out)
	}
	workflow, err := os.ReadFile("../.github/workflows/race-pr.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"ci-pr-race-budgets.py", "matrix.job_minutes", "matrix.test_minutes"} {
		if !strings.Contains(string(workflow), marker) {
			t.Errorf("workflow missing %s", marker)
		}
	}
}

func TestCIPRRaceMeasuredBudgets(t *testing.T) {
	cmd := exec.Command("python3", "ci-pr-race-budgets.py", "ci-pr-race-budgets.json")
	cmd.Stdin = strings.NewReader(`["github.com/glade-sh/glade/internal/apextest","github.com/glade-sh/glade/internal/gladecli","github.com/glade-sh/glade/internal/storage"]`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("measured budgets: %v: %s", err, out)
	}
	var matrix struct {
		Include []struct {
			Package     string `json:"package"`
			TestMinutes int    `json:"test_minutes"`
			JobMinutes  int    `json:"job_minutes"`
		} `json:"include"`
	}
	if err := json.Unmarshal(out, &matrix); err != nil {
		t.Fatal(err)
	}
	if len(matrix.Include) != 3 || matrix.Include[0].Package != "github.com/glade-sh/glade/internal/apextest" || matrix.Include[1].Package != "github.com/glade-sh/glade/internal/gladecli" || matrix.Include[2].Package != "github.com/glade-sh/glade/internal/storage" || matrix.Include[0].TestMinutes != 19 || matrix.Include[0].JobMinutes != 24 || matrix.Include[1].TestMinutes != 16 || matrix.Include[1].JobMinutes != 21 || matrix.Include[2].TestMinutes != 9 || matrix.Include[2].JobMinutes != 11 {
		t.Fatalf("unexpected measured budgets: %s", out)
	}
	data, err := os.ReadFile("../.github/workflows/race-pr.yml")
	if err != nil {
		t.Fatal(err)
	}
	regex := `regex='Concurren|Parallel|Race|Goroutine|Cancel|Daemon|Watch|Listener|Shutdown|Graceful|Interrupt|Pool|Worker|Heartbeat|Debounce|Singleflight|Reentrant|ThreadSafe|Overlap'`
	if !strings.Contains(string(data), regex) {
		t.Fatal("race test selection changed")
	}
}

func TestCIPRRaceBudgetsRejectInvalidSamples(t *testing.T) {
	for _, data := range []string{
		`{"default":{"testMinutes":9,"jobMinutes":9},"samples":[]}`,
		`{"default":{"testMinutes":9,"jobMinutes":11},"samples":[{"package":"x","testSeconds":-1,"multiplier":2,"setupSeconds":1,"marginSeconds":1}]}`,
		`{"default":{"testMinutes":9,"jobMinutes":11},"samples":[{"package":"x","testSeconds":1,"multiplier":2,"setupSeconds":1,"marginSeconds":1},{"package":"x","testSeconds":1,"multiplier":2,"setupSeconds":1,"marginSeconds":1}]}`,
	} {
		path := filepath.Join(t.TempDir(), "budgets.json")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("python3", "ci-pr-race-budgets.py", path)
		cmd.Stdin = strings.NewReader(`["x"]`)
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("invalid budget accepted: %s", out)
		}
	}
}

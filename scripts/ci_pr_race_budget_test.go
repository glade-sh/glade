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

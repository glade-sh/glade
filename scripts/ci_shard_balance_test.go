package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestCIShardCountsBalanceOnCheckedInDurations guards the CI shard counts
// Each package's checked-in duration table, planned at the count CI
// uses, must put no shard more than 20% over the mean estimate. Rerun after a
// table refresh; a failure means the count or the table needs another look.
func TestCIShardCountsBalanceOnCheckedInDurations(t *testing.T) {
	for _, tc := range []struct {
		pkg    string
		shards int
	}{
		{"apextest", 16},
		{"vm", 3},
		{"gladecli", 2},
		{"sema", 2},
		{"visualforce", 4},
	} {
		t.Run(tc.pkg, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "internal", tc.pkg, "testdata", "shard_durations.json"))
			if err != nil {
				t.Fatal(err)
			}
			var table struct {
				Package string `json:"package"`
				Tests   []struct {
					Name           string `json:"name"`
					DurationMillis *int64 `json:"durationMillis"`
				} `json:"tests"`
			}
			if err := json.Unmarshal(raw, &table); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			history := map[string]any{"version": 1, "package": table.Package, "complete": true, "tests": table.Tests}
			historyJSON, _ := json.Marshal(history)
			names := make([]string, 0, len(table.Tests))
			for _, test := range table.Tests {
				names = append(names, test.Name)
			}
			historyPath, listPath := filepath.Join(dir, "history.json"), filepath.Join(dir, "tests.txt")
			if err := os.WriteFile(historyPath, historyJSON, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(listPath, []byte(strings.Join(names, "\n")+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "run", "./scripts/internal/cishard", "--package", table.Package,
				"--shards", strconv.Itoa(tc.shards), "--tests", listPath, "--history", historyPath)
			cmd.Dir = ".."
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("cishard: %v", err)
			}
			var plan struct {
				HistoryUsed bool `json:"historyUsed"`
				Shards      []struct {
					Tests    []string `json:"tests"`
					Estimate int64    `json:"estimatedDurationMillis"`
				} `json:"shards"`
			}
			if err := json.Unmarshal(out, &plan); err != nil {
				t.Fatal(err)
			}
			if !plan.HistoryUsed || len(plan.Shards) != tc.shards {
				t.Fatalf("history used %v, %d shards", plan.HistoryUsed, len(plan.Shards))
			}
			var total, max int64
			count := 0
			for _, shard := range plan.Shards {
				total += shard.Estimate
				count += len(shard.Tests)
				if shard.Estimate > max {
					max = shard.Estimate
				}
			}
			if count != len(names) || total == 0 {
				t.Fatalf("planned %d of %d tests, total %d", count, len(names), total)
			}
			if mean := float64(total) / float64(tc.shards); float64(max) > 1.2*mean {
				t.Errorf("max shard %.0fs exceeds 1.2x mean %.0fs", float64(max)/1000, mean/1000)
			}
		})
	}
}

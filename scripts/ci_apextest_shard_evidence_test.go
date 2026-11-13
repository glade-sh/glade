package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCICase319TwelveShardEvidenceValidation(t *testing.T) {
	names := make([]string, 12)
	shards := make([]map[string]any, 12)
	for i := range names {
		names[i] = fmt.Sprintf("TestBootstrap%02d", i)
		shards[i] = map[string]any{"index": i, "tests": []string{names[i]}, "estimatedDurationMillis": 0, "regex": "^(?:" + names[i] + ")$"}
	}
	plan, err := json.Marshal(map[string]any{"version": 1, "package": fixturePackage, "historyUsed": false, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"valid", "missing", "mismatched"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			dirs := writePlannedHistoryEvidence(t, root, fixturePackage, names, plan, func(i int, _ string) float64 { return float64(i + 1) })
			switch mode {
			case "missing":
				if err := os.Remove(filepath.Join(dirs[11], "events.json")); err != nil {
					t.Fatal(err)
				}
			case "mismatched":
				if err := os.WriteFile(filepath.Join(dirs[11], "discovery.txt"), []byte("TestOther\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(root, "history.json")
			args := append([]string{"ci-go-test.sh", "apex-history-refresh", output}, dirs...)
			cmd := exec.Command("bash", args...)
			cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME=push")
			out, err := cmd.CombinedOutput()
			if mode == "valid" {
				if err != nil {
					t.Fatalf("valid evidence rejected: %v %s", err, out)
				}
				data, err := os.ReadFile(output)
				if err != nil || !strings.Contains(string(data), `"durationMillis": 12000`) {
					t.Fatalf("missing observed duration: %s %v", data, err)
				}
			} else {
				if err == nil {
					t.Fatalf("invalid evidence accepted: %s", out)
				}
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatalf("invalid evidence published history: %v", err)
				}
			}
		})
	}
}

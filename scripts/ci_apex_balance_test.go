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

// Use fixed observations after the owned project has executed so the CI policy
// test is independent of local clock granularity and competing runner load.
func assertApexProjectHistoryRebalances(t *testing.T, planner, output string, dirs []string) {
	t.Helper()
	weights := map[string]int64{}
	for index, dir := range dirs {
		path := filepath.Join(dir, "events.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var events strings.Builder
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			var event map[string]any
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				t.Fatal(err)
			}
			if name, ok := event["Test"].(string); ok && event["Action"] == "pass" {
				seconds := 1
				if index == 0 {
					seconds = 4
				}
				event["Elapsed"] = seconds
				weights[name] = int64(seconds * 1000)
			}
			encoded, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintln(&events, string(encoded))
		}
		if err := os.WriteFile(path, []byte(events.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GITHUB_EVENT_NAME", "push")
	// The same skew under a history-driven plan is a planner failure.
	setPlanHistoryUsed(t, dirs, true)
	if out, err := runNShardHistory(t, "cli", output, dirs); err == nil || !strings.Contains(out, "exceeds 1.5x") {
		t.Fatalf("main refresh must reject the skewed history-planned project: %v\n%s", err, out)
	}
	// The project ran from the deterministic fallback plan, so main keeps its
	// measured durations with an advisory.
	setPlanHistoryUsed(t, dirs, false)
	if out, err := runNShardHistory(t, "cli", output, dirs); err != nil || !strings.Contains(out, "timing advisory:") {
		t.Fatalf("main must retain measured fallback-planned project durations: %v\n%s", err, out)
	}
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	if out, err := runNShardHistory(t, "cli", output, dirs); err != nil || !strings.Contains(out, "timing advisory:") {
		t.Fatalf("PR must retain measured project durations: %v\n%s", err, out)
	}
	data, err := exec.Command(planner, "--package", fixturePackage, "--shards", "6",
		"--tests", filepath.Join(dirs[0], "discovery.txt"), "--history", output).Output()
	var plan struct {
		HistoryUsed bool
		Shards      []struct {
			Tests  []string
			Millis int64 `json:"estimatedDurationMillis"`
		}
	}
	if err != nil || json.Unmarshal(data, &plan) != nil || !plan.HistoryUsed || len(plan.Shards) != 6 {
		t.Fatalf("measured project replan: %v %s", err, data)
	}
	seen := map[string]bool{}
	for _, shard := range plan.Shards {
		var observed int64
		for _, name := range shard.Tests {
			if seen[name] || weights[name] == 0 {
				t.Fatalf("replanned union has duplicate or unknown test %s", name)
			}
			seen[name] = true
			observed += weights[name]
		}
		if observed != shard.Millis || shard.Millis > 4000 {
			t.Fatalf("measured replan must reduce the 8s slow shard to at most 4s: %s", data)
		}
	}
	if len(seen) != len(weights) || len(seen) != 12 {
		t.Fatalf("measured replan lost project coverage: %d/%d", len(seen), len(weights))
	}
}

func nShardHistoryFixture(t *testing.T, elapsed []float64) (string, []string) {
	t.Helper()
	root := t.TempDir()
	names := make([]string, len(elapsed))
	shards := make([]map[string]any, len(elapsed))
	for i := range names {
		names[i] = fmt.Sprintf("TestBalance%02d", i)
		shards[i] = map[string]any{"index": i, "tests": []string{names[i]},
			"estimatedDurationMillis": 0, "regex": "^(?:" + names[i] + ")$"}
	}
	plan := map[string]any{"version": 1, "package": fixturePackage, "historyUsed": false, "shards": shards}
	dirs := make([]string, len(elapsed))
	for i := range dirs {
		dirs[i] = filepath.Join(root, fmt.Sprint(i))
		if err := os.Mkdir(dirs[i], 0700); err != nil {
			t.Fatal(err)
		}
		writeJSONFixture(t, filepath.Join(dirs[i], "plan.json"), plan)
		writeJSONFixture(t, filepath.Join(dirs[i], "selected-shard.json"), shards[i])
		writeJSONFixture(t, filepath.Join(dirs[i], "validation-summary.json"), map[string]any{
			"valid": true, "expected": []string{names[i]}, "passed": []string{names[i]}, "errors": []string{},
		})
		for file, data := range map[string]string{
			"discovery.txt": strings.Join(names, "\n") + "\n",
			"events.json": fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%g}\n",
				fixturePackage, names[i], elapsed[i]),
		} {
			if err := os.WriteFile(filepath.Join(dirs[i], file), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return filepath.Join(root, "history.json"), dirs
}

func runNShardHistory(t *testing.T, entry, output string, dirs []string) (string, error) {
	t.Helper()
	args := append([]string{"ci-go-test.sh", "apex-history-refresh", output}, dirs...)
	if len(dirs) == 2 {
		args = []string{"ci-go-test.sh", "apex-history-refresh", dirs[0], dirs[1], output}
	}
	if entry == "sourced" {
		args = append([]string{"-c", `source "$1"; shift; refresh_apextest_history "$@"`, "history", "ci-go-test.sh"}, args[2:]...)
	} else if entry == "generic" {
		args = append([]string{"-c", `source "$1"; shift; refresh_test_history "$@"`, "history", "ci-go-test.sh",
			fixturePackage, "Apex", "apextest", "0", fmt.Sprint(len(dirs)), output}, dirs...)
	}
	out, err := exec.Command("bash", args...).CombinedOutput()
	return string(out), err
}

func TestCIApexNShardBalanceTimingRoutes(t *testing.T) {
	for _, count := range []int{2, 3, 4, 6, 8} {
		entries := []string{"generic"}
		if count == 2 || count == 6 {
			entries = append(entries, "cli", "sourced")
		}
		for _, entry := range entries {
			for _, event := range []string{"pull_request", "push", "schedule", "workflow_dispatch", ""} {
				for _, timing := range []string{"zero", "balanced", "boundary", "above", "above-fallback"} {
					t.Run(fmt.Sprintf("%d/%s/%s/%s", count, entry, event, timing), func(t *testing.T) {
						fallback := timing == "above-fallback"
						if fallback {
							timing = "above"
						}
						t.Setenv("GITHUB_EVENT_NAME", event)
						elapsed := make([]float64, count)
						for i := range elapsed {
							if timing != "zero" {
								elapsed[i] = 1
							}
						}
						if timing == "boundary" || timing == "above" {
							elapsed[0] = 1.5
							if count == 2 {
								elapsed[0] = 3 // Median is the mean for two shards.
							}
							if timing == "above" {
								elapsed[0] += 0.001
							}
						}
						output, dirs := nShardHistoryFixture(t, elapsed)
						setPlanHistoryUsed(t, dirs, !fallback)
						if err := os.WriteFile(output, []byte("stale"), 0600); err != nil {
							t.Fatal(err)
						}
						out, err := runNShardHistory(t, entry, output, dirs)
						// A fallback plan splits by count, so its timing is advisory.
						wantErr := timing == "above" && event != "pull_request" && !fallback
						if (err != nil) != wantErr {
							t.Fatalf("refresh error = %v, wantErr %v\n%s", err, wantErr, out)
						}
						if strings.Contains(out, "timing advisory:") != (timing == "above" && !wantErr) {
							t.Fatalf("incorrect timing advisory\n%s", out)
						}
						if timing == "above" && !strings.Contains(out, fmt.Sprintf("1.5x %d-shard median", count)) {
							t.Fatalf("balance diagnostic must identify the shard count\n%s", out)
						}
						_, statErr := os.Stat(output)
						if wantErr && !os.IsNotExist(statErr) || !wantErr && statErr != nil {
							t.Fatalf("history publication: %v, wantErr %v", statErr, wantErr)
						}
					})
				}
			}
		}
	}
}

func TestCIApexSixShardBalanceAdvisoryPreservesUnionGuard(t *testing.T) {
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	for _, entry := range []string{"cli", "sourced"} {
		for _, mutation := range []string{"missing", "duplicate", "extra"} {
			t.Run(entry+"/"+mutation, func(t *testing.T) {
				output, dirs := nShardHistoryFixture(t, []float64{10, 1, 1, 1, 1, 1})
				path := filepath.Join(dirs[5], "events.json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				switch mutation {
				case "missing":
					data = nil
				case "duplicate":
					data = append(data, data...)
				case "extra":
					data = append(data, []byte(strings.ReplaceAll(string(data), "TestBalance05", "TestExtra"))...)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				out, err := runNShardHistory(t, entry, output, dirs)
				if err == nil || !strings.Contains(out, "refresh rejected:") || strings.Contains(out, "timing advisory:") {
					t.Fatalf("invalid union bypassed evidence validation: %v\n%s", err, out)
				}
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatalf("invalid union published history: %v", err)
				}
			})
		}
	}
}

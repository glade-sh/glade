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

func lwcHistoryFixture(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	names := make([]string, 16)
	shards := make([]map[string]any, 8)
	for i := range names {
		names[i] = fmt.Sprintf("TestFamily%02d", i)
	}
	for i := range shards {
		tests := names[i*2 : i*2+2]
		shards[i] = map[string]any{"index": i, "tests": tests,
			"estimatedDurationMillis": 0, "regex": "^(?:" + strings.Join(tests, "|") + ")$"}
	}
	plan := map[string]any{"version": 1, "package": lwcCompileFixturePackage, "historyUsed": false, "shards": shards}
	dirs := make([]string, 8)
	for i := range dirs {
		dirs[i] = filepath.Join(root, fmt.Sprintf("shard-%d", i))
		if err := os.Mkdir(dirs[i], 0o700); err != nil {
			t.Fatal(err)
		}
		write := func(name, data string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(dirs[i], name), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		write("discovery.txt", strings.Join(names, "\n")+"\n")
		writeJSONFixture(t, filepath.Join(dirs[i], "plan.json"), plan)
		writeJSONFixture(t, filepath.Join(dirs[i], "selected-shard.json"), shards[i])
		tests := names[i*2 : i*2+2]
		writeJSONFixture(t, filepath.Join(dirs[i], "validation-summary.json"), map[string]any{
			"valid": true, "expected": tests, "passed": tests, "errors": []string{}, "deferred": []string{}})
		var events strings.Builder
		for j, name := range tests {
			// Deliberately skewed first-run evidence must still seed balancing.
			elapsed := 1.0
			if i == 0 {
				elapsed = float64(100 - j*10)
			}
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%g}\n", lwcCompileFixturePackage, name, elapsed)
		}
		fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":1}\n", lwcCompileFixturePackage)
		write("events.json", events.String())
	}
	return filepath.Join(root, "history.json"), dirs
}

func runLWCHistory(t *testing.T, output string, dirs []string) (string, error) {
	t.Helper()
	args := append([]string{"ci-go-test.sh", "lwc-compile-history-refresh", output}, dirs...)
	out, err := exec.Command("bash", args...).CombinedOutput()
	return string(out), err
}

func TestCILWCHistoryBalancesCompleteEightShardEvidence(t *testing.T) {
	output, dirs := lwcHistoryFixture(t)
	if out, err := runLWCHistory(t, output, dirs); err != nil {
		t.Fatalf("refresh failed: %v\n%s", err, out)
	}
	var history struct {
		Complete bool `json:"complete"`
		Tests    []struct {
			Name   string `json:"name"`
			Millis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	data, err := os.ReadFile(output)
	if err != nil || json.Unmarshal(data, &history) != nil || !history.Complete || len(history.Tests) != 16 {
		t.Fatalf("incomplete history: %v %s", err, data)
	}
	if history.Tests[0].Millis != 100000 || history.Tests[1].Millis != 90000 {
		t.Fatal("history did not retain observed durations")
	}
	cmd := exec.Command(realGoCommand(t), "run", "./internal/cishard", "--package", lwcCompileFixturePackage,
		"--shards", "8", "--tests", filepath.Join(dirs[0], "discovery.txt"), "--history", output)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("planner rejected refreshed history: %v", err)
	}
	var plan struct {
		HistoryUsed bool `json:"historyUsed"`
		Shards      []struct {
			Tests  []string `json:"tests"`
			Millis int64    `json:"estimatedDurationMillis"`
		} `json:"shards"`
	}
	if err := json.Unmarshal(out, &plan); err != nil || !plan.HistoryUsed || len(plan.Shards) != 8 {
		t.Fatalf("planner did not use history: %v %s", err, out)
	}
	seen := map[string]bool{}
	for _, shard := range plan.Shards {
		if shard.Millis > 100000 {
			t.Fatalf("planner left the two slow tests together: %s", out)
		}
		for _, name := range shard.Tests {
			if seen[name] {
				t.Fatalf("duplicate planned test %s", name)
			}
			seen[name] = true
		}
	}
	if len(seen) != 16 {
		t.Fatalf("planned union has %d tests, want 16", len(seen))
	}
}

func TestCILWCHistoryRejectsIncompleteOrInconsistentUnion(t *testing.T) {
	for _, tc := range []struct{ name, file, old, replacement string }{
		{"last plan differs", "plan.json", `"historyUsed": false`, `"historyUsed": true`},
		{"last discovery differs", "discovery.txt", "TestFamily15", "TestFamily99"},
		{"last selection differs", "selected-shard.json", "TestFamily15", "TestFamily99"},
		{"missing terminal", "events.json", `"TestFamily15"`, `"TestUnknown"`},
		{"skip terminal", "events.json", `"Action":"pass"`, `"Action":"skip"`},
		{"failed terminal", "events.json", `"Action":"pass"`, `"Action":"fail"`},
		{"missing package result", "events.json", fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":1}\n", lwcCompileFixturePackage), ""},
		{"invalid summary", "validation-summary.json", `"valid": true`, `"valid": false`},
		{"unexpected deferral", "validation-summary.json", `"deferred": []`, `"deferred": ["TestFamily15"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, dirs := lwcHistoryFixture(t)
			path := filepath.Join(dirs[7], tc.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), tc.old, tc.replacement, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			// Failed refreshes must also remove an old output, never publish it.
			if err := os.WriteFile(output, []byte("stale"), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := runLWCHistory(t, output, dirs); err == nil {
				t.Fatalf("accepted corrupt evidence: %s", out)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("stale history survived: %v", err)
			}
		})
	}
}

func TestCILWCHistoryRechecksNestedResults(t *testing.T) {
	for _, action := range []string{"skip", "fail"} {
		t.Run(action, func(t *testing.T) {
			output, dirs := lwcHistoryFixture(t)
			path := filepath.Join(dirs[7], "events.json")
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = fmt.Fprintf(file, "{\"Action\":%q,\"Package\":%q,\"Test\":\"TestFamily15/BrowserRuntime\"}\n", action, lwcCompileFixturePackage)
			if closeErr := file.Close(); err != nil || closeErr != nil {
				t.Fatalf("append result: %v %v", err, closeErr)
			}
			if out, err := runLWCHistory(t, output, dirs); err == nil {
				t.Fatalf("accepted nested %s under passing parent: %s", action, out)
			}
		})
	}
}

func TestCILWCHistoryWorkflowSharesInputAndRequiresUnion(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	input, matrix, refresh := jobs["lwc-compile-history-input"], jobs["test"], jobs["lwc-compile-history"]
	for _, marker := range []string{"needs: [lwc-compile-history-input, visualforce-history-input, test-binaries]", "name: lwc-compile-history-input", "CI_LWC_COMPILE_HISTORY_PATH", "name: Download shared LWC duration history"} {
		if !strings.Contains(matrix, marker) {
			t.Errorf("matrix does not consume shared input: missing %q", marker)
		}
	}
	if strings.Contains(matrix, "lwc-compile-duration-history-v1") {
		t.Error("workers must not independently restore mutable timing inputs")
	}
	for _, marker := range []string{"actions: read", "-f head_sha=\"$CI_DURATION_HISTORY_CACHE_REF\"", "-f status=success", "run-id: ${{ steps.predecessor.outputs.run_id }}", "go-test-lwc-compile-shard-{0..7}", "using deterministic scheduling fallback"} {
		if !strings.Contains(input, marker) {
			t.Errorf("validated predecessor bootstrap missing %q", marker)
		}
	}
	for _, marker := range []string{"needs: test", "if: ${{ success() }}", "pattern: go-test-lwc-compile-shard-*", "lwc-compile-history-refresh", "go-test-lwc-compile-shard-{0..7}", "actions/cache/save@"} {
		if !strings.Contains(refresh, marker) {
			t.Errorf("validated eight-shard publisher missing %q", marker)
		}
	}
	if !strings.Contains(jobs["required-ci"], "      - lwc-compile-history\n") {
		t.Error("required CI must depend on successful complete LWC union validation")
	}
}

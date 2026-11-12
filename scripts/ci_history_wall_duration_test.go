package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A top-level test that fans out into parallel subtests reports an Elapsed
// that excludes the time it waits for them. The Apex planner must weigh such a
// test by the wall time between its run and terminal events.

type wallEvent struct {
	action  string
	test    string
	at      float64 // seconds after the fixture epoch; negative omits Time
	elapsed float64
}

func wallEventLines(packageName string, events []wallEvent) string {
	epoch := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var out strings.Builder
	for _, event := range events {
		row := map[string]any{"Action": event.action, "Package": packageName, "Test": event.test}
		if event.at >= 0 {
			row["Time"] = epoch.Add(time.Duration(event.at * float64(time.Second))).Format(time.RFC3339Nano)
		}
		if event.action == "pass" {
			row["Elapsed"] = event.elapsed
		}
		data, _ := json.Marshal(row)
		out.Write(data)
		out.WriteByte('\n')
	}
	return out.String()
}

// wallHistoryFixture writes one shard directory per entry of shardEvents. Each
// shard owns the single test TestWallNN.
func wallHistoryFixture(t *testing.T, packageName string, historyUsed bool, shardEvents [][]wallEvent) (string, []string) {
	t.Helper()
	root := t.TempDir()
	names := make([]string, len(shardEvents))
	shards := make([]map[string]any, len(shardEvents))
	for i := range names {
		names[i] = fmt.Sprintf("TestWall%02d", i)
		shards[i] = map[string]any{"index": i, "tests": []string{names[i]},
			"estimatedDurationMillis": 0, "regex": "^(?:" + names[i] + ")$"}
	}
	plan := map[string]any{"version": 1, "package": packageName, "historyUsed": historyUsed, "shards": shards}
	dirs := make([]string, len(shardEvents))
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
			"events.json":   wallEventLines(packageName, shardEvents[i]),
		} {
			if err := os.WriteFile(filepath.Join(dirs[i], file), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return filepath.Join(root, "history.json"), dirs
}

func runWallHistory(t *testing.T, packageName, label, prefix, output string, dirs []string) (string, error) {
	t.Helper()
	args := append([]string{"-c", `source "$1"; shift; refresh_test_history "$@"`, "history", "ci-go-test.sh",
		packageName, label, prefix, "0", fmt.Sprint(len(dirs)), output}, dirs...)
	out, err := exec.Command("bash", args...).CombinedOutput()
	return string(out), err
}

func readWallHistory(t *testing.T, path string) map[string]int64 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Tests []struct {
			Name   string `json:"name"`
			Millis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, row := range history.Tests {
		out[row.Name] = row.Millis
	}
	return out
}

// parallelParent is the measured shape: the parent's Elapsed is near zero and
// its parallel subtests fill the wall interval.
func parallelParent(name string, wall float64) []wallEvent {
	return []wallEvent{
		{action: "run", test: name, at: 10},
		{action: "run", test: name + "/api", at: 10.1},
		{action: "pause", test: name + "/api", at: 10.2},
		{action: "cont", test: name + "/api", at: 10.3},
		{action: "pass", test: name + "/api", at: 10 + wall - 0.1, elapsed: wall - 0.4},
		{action: "pass", test: name, at: 10 + wall, elapsed: 0.25},
	}
}

func sequentialTest(name string, seconds float64) []wallEvent {
	return []wallEvent{
		{action: "run", test: name, at: 10},
		{action: "pass", test: name, at: 10 + seconds, elapsed: seconds},
	}
}

func TestCIApexHistoryWeighsParallelSubtestParentsByWallTime(t *testing.T) {
	t.Setenv("GITHUB_EVENT_NAME", "push")
	output, dirs := wallHistoryFixture(t, fixturePackage, false, [][]wallEvent{
		parallelParent("TestWall00", 240),
		sequentialTest("TestWall01", 240),
	})
	if out, err := runWallHistory(t, fixturePackage, "Apex", "apextest", output, dirs); err != nil {
		t.Fatalf("refresh: %v\n%s", err, out)
	}
	got := readWallHistory(t, output)
	if got["TestWall00"] != 240000 || got["TestWall01"] != 240000 {
		t.Fatalf("durations = %v, want both 240000 (parent Elapsed was 0.25s)", got)
	}
}

func TestCIApexHistoryWallTimeFallsBackToElapsed(t *testing.T) {
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	cases := map[string][]wallEvent{
		// A top-level parallel test waits for the sequential tests; only its
		// Elapsed is its own cost.
		"top-level pause": {
			{action: "run", test: "TestWall00", at: 10},
			{action: "pause", test: "TestWall00", at: 10.1},
			{action: "cont", test: "TestWall00", at: 300},
			{action: "pass", test: "TestWall00", at: 303, elapsed: 3},
		},
		"no timestamps": {
			{action: "run", test: "TestWall00", at: -1},
			{action: "pass", test: "TestWall00", at: -1, elapsed: 3},
		},
		"no run event":              {{action: "pass", test: "TestWall00", at: 500, elapsed: 3}},
		"terminal before run":       {{action: "run", test: "TestWall00", at: 500}, {action: "pass", test: "TestWall00", at: 100, elapsed: 3}},
		"wall shorter than elapsed": {{action: "run", test: "TestWall00", at: 10}, {action: "pass", test: "TestWall00", at: 11, elapsed: 3}},
	}
	for name, events := range cases {
		t.Run(name, func(t *testing.T) {
			output, dirs := wallHistoryFixture(t, fixturePackage, false, [][]wallEvent{events, sequentialTest("TestWall01", 3)})
			if out, err := runWallHistory(t, fixturePackage, "Apex", "apextest", output, dirs); err != nil {
				t.Fatalf("refresh: %v\n%s", err, out)
			}
			if got := readWallHistory(t, output); got["TestWall00"] != 3000 {
				t.Fatalf("durations = %v, want TestWall00 3000 from Elapsed", got)
			}
		})
	}
	t.Run("malformed timestamp", func(t *testing.T) {
		output, dirs := wallHistoryFixture(t, fixturePackage, false, [][]wallEvent{sequentialTest("TestWall00", 3), sequentialTest("TestWall01", 3)})
		path := filepath.Join(dirs[0], "events.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		broken := strings.Replace(string(data), `"Time":"2026-`, `"Time":"not-a-time-2026-`, 1)
		if broken == string(data) {
			t.Fatal("fixture has no timestamp to break")
		}
		if err := os.WriteFile(path, []byte(broken), 0600); err != nil {
			t.Fatal(err)
		}
		if out, err := runWallHistory(t, fixturePackage, "Apex", "apextest", output, dirs); err != nil {
			t.Fatalf("refresh: %v\n%s", err, out)
		}
		if got := readWallHistory(t, output); got["TestWall00"] != 3000 {
			t.Fatalf("durations = %v, want TestWall00 3000 from Elapsed", got)
		}
	})
}

// The balance gate must see the same weights the planner will use: a shard
// whose parents hide four minutes behind parallel subtests is not balanced.
func TestCIApexHistoryBalanceGateUsesWallTime(t *testing.T) {
	events := [][]wallEvent{
		parallelParent("TestWall00", 240),
		sequentialTest("TestWall01", 1),
		sequentialTest("TestWall02", 1),
	}
	t.Setenv("GITHUB_EVENT_NAME", "push")
	output, dirs := wallHistoryFixture(t, fixturePackage, true, events)
	if out, err := runWallHistory(t, fixturePackage, "Apex", "apextest", output, dirs); err == nil || !strings.Contains(out, "shard 0 elapsed") || !strings.Contains(out, "exceeds 1.5x") {
		t.Fatalf("history-planned skew hidden behind parallel subtests must be rejected: %v\n%s", err, out)
	}
	output, dirs = wallHistoryFixture(t, fixturePackage, false, events)
	out, err := runWallHistory(t, fixturePackage, "Apex", "apextest", output, dirs)
	if err != nil || !strings.Contains(out, "timing advisory:") {
		t.Fatalf("fallback-planned skew must stay advisory: %v\n%s", err, out)
	}
	if got := readWallHistory(t, output); got["TestWall00"] != 240000 {
		t.Fatalf("fallback-planned refresh must keep wall durations: %v", got)
	}
}

// Other packages keep their measured Elapsed weights and balance policy.
func TestCIHistoryWallTimeIsLimitedToApex(t *testing.T) {
	const sema = "github.com/glade-sh/glade/internal/sema"
	t.Setenv("GITHUB_EVENT_NAME", "push")
	output, dirs := wallHistoryFixture(t, sema, true, [][]wallEvent{
		parallelParent("TestWall00", 240),
		sequentialTest("TestWall01", 0.25),
	})
	if out, err := runWallHistory(t, sema, "sema", "sema", output, dirs); err != nil {
		t.Fatalf("sema refresh: %v\n%s", err, out)
	}
	if got := readWallHistory(t, output); got["TestWall00"] != 250 || got["TestWall01"] != 250 {
		t.Fatalf("sema durations = %v, want Elapsed-based 250 each", got)
	}
}

// Histories recorded with Elapsed weights must not drive a plan that the
// wall-time balance gate then judges: the Apex cache key names the metric.
func TestCIApexHistoryCacheKeyNamesWallMetric(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	keys := 0
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "apextest-duration-history-v1") {
			continue
		}
		keys++
		if !strings.Contains(line, "apextest-duration-history-v1-wall-${{ runner.os }}") {
			t.Errorf("Apex history cache key does not name the wall metric: %s", strings.TrimSpace(line))
		}
	}
	if keys != 2 {
		t.Fatalf("Apex history cache keys = %d, want restore and save", keys)
	}
}

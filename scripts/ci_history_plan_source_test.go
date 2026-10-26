package scripts

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setPlanHistoryUsed records whether the shard plan behind the evidence came
// from duration history or from the planner's deterministic fallback.
func setPlanHistoryUsed(t *testing.T, dirs []string, used bool) {
	t.Helper()
	for _, dir := range dirs {
		path := filepath.Join(dir, "plan.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var plan map[string]any
		if err := json.Unmarshal(data, &plan); err != nil {
			t.Fatal(err)
		}
		if _, ok := plan["historyUsed"].(bool); !ok {
			t.Fatalf("plan has no boolean historyUsed: %s", data)
		}
		plan["historyUsed"] = used
		writeJSONFixture(t, path, plan)
	}
}

// writePlannedHistoryEvidence writes passing shard evidence for a planner
// result, with each test's observed elapsed time.
func writePlannedHistoryEvidence(t *testing.T, root, packageName string, names []string, planData []byte, elapsed func(shard int, name string) float64) []string {
	t.Helper()
	var plan struct {
		Shards []map[string]any `json:"shards"`
	}
	if err := json.Unmarshal(planData, &plan); err != nil {
		t.Fatalf("planner output: %v\n%s", err, planData)
	}
	var raw map[string]any
	if err := json.Unmarshal(planData, &raw); err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, len(plan.Shards))
	for index, shard := range plan.Shards {
		dirs[index] = filepath.Join(root, fmt.Sprintf("shard-%d", index))
		if err := os.MkdirAll(dirs[index], 0o700); err != nil {
			t.Fatal(err)
		}
		var tests []string
		for _, name := range shard["tests"].([]any) {
			tests = append(tests, name.(string))
		}
		writeJSONFixture(t, filepath.Join(dirs[index], "plan.json"), raw)
		writeJSONFixture(t, filepath.Join(dirs[index], "selected-shard.json"), shard)
		writeJSONFixture(t, filepath.Join(dirs[index], "validation-summary.json"), map[string]any{
			"valid": true, "expected": tests, "passed": tests, "errors": []string{},
		})
		var events strings.Builder
		for _, name := range tests {
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%g}\n", packageName, name, elapsed(index, name))
		}
		for file, data := range map[string]string{
			"discovery.txt": strings.Join(names, "\n") + "\n",
			"events.json":   events.String(),
		} {
			if err := os.WriteFile(filepath.Join(dirs[index], file), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dirs
}

func TestCIDurationHistoryFallbackPlanTimingAdvisory(t *testing.T) {
	// The skew observed on a main push of the two-shard sema lane, planned
	// without duration history.
	elapsed := [2]float64{193.89, 51.65}
	for _, route := range []string{"sema", "apex"} {
		for _, entry := range []string{"cli", "sourced"} {
			for _, source := range []string{"fallback", "history"} {
				for _, event := range []string{"push", "schedule", "workflow_dispatch", "", "pull_request", "pull_request_target"} {
					t.Run(fmt.Sprintf("%s/%s/%s/%q", route, entry, source, event), func(t *testing.T) {
						t.Setenv("GITHUB_EVENT_NAME", event)
						packageName := fixturePackage
						if route == "sema" {
							packageName = semaFixturePackage
						}
						fixture := durationHistoryRouteFixture(t, packageName, elapsed)
						setPlanHistoryUsed(t, fixture.shards[:], source == "history")
						out, err := runDurationHistoryRoute(t, route, entry, fixture)
						wantErr := source == "history" && event != "pull_request"
						if (err != nil) != wantErr {
							t.Fatalf("refresh error = %v, wantErr %v\n%s", err, wantErr, out)
						}
						if wantErr {
							if !strings.Contains(out, "exceeds 1.5x 2-shard median") || strings.Contains(out, "timing advisory:") {
								t.Fatalf("history-planned imbalance must stay fatal outside PRs\n%s", out)
							}
							if _, err := os.Stat(fixture.output); !os.IsNotExist(err) {
								t.Fatalf("rejected refresh left history: %v", err)
							}
							return
						}
						if !strings.Contains(out, "::warning::") || !strings.Contains(out, "duration history timing advisory:") || !strings.Contains(out, "exceeds 1.5x 2-shard median") {
							t.Fatalf("accepted imbalance must print the timing advisory\n%s", out)
						}
						var history struct {
							Package  string `json:"package"`
							Complete bool   `json:"complete"`
							Tests    []struct {
								DurationMillis int64 `json:"durationMillis"`
							} `json:"tests"`
						}
						data, err := os.ReadFile(fixture.output)
						if err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal(data, &history); err != nil || history.Package != packageName || !history.Complete || len(history.Tests) != 2 {
							t.Fatalf("invalid refreshed history: %s err=%v", data, err)
						}
						for index, test := range history.Tests {
							if test.DurationMillis != int64(math.Round(elapsed[index]*1000)) {
								t.Fatalf("history lost observed duration: %s", data)
							}
						}
					})
				}
			}
		}
	}
}

func TestCIDurationHistoryFallbackPlanKeepsEvidenceValidation(t *testing.T) {
	t.Setenv("GITHUB_EVENT_NAME", "push")
	for _, route := range []string{"sema", "apex"} {
		for _, invalid := range []string{"missing terminal", "failed terminal", "plan mismatch"} {
			t.Run(route+"/"+invalid, func(t *testing.T) {
				packageName := fixturePackage
				if route == "sema" {
					packageName = semaFixturePackage
				}
				fixture := durationHistoryRouteFixture(t, packageName, [2]float64{193.89, 51.65})
				switch invalid {
				case "missing terminal":
					if err := os.WriteFile(filepath.Join(fixture.shards[1], "events.json"), nil, 0o600); err != nil {
						t.Fatal(err)
					}
				case "failed terminal":
					data := fmt.Sprintf(`{"Action":"fail","Package":%q,"Test":"TestHistory001","Elapsed":51.65}`+"\n", packageName)
					if err := os.WriteFile(filepath.Join(fixture.shards[1], "events.json"), []byte(data), 0o600); err != nil {
						t.Fatal(err)
					}
				case "plan mismatch":
					// One shard claims history planning; the plans must still match.
					setPlanHistoryUsed(t, fixture.shards[1:], true)
				}
				out, err := runDurationHistoryRoute(t, route, "cli", fixture)
				if err == nil || !strings.Contains(out, "duration history refresh rejected:") || strings.Contains(out, "timing advisory:") {
					t.Fatalf("fallback plan bypassed evidence validation: %v\n%s", err, out)
				}
				if _, err := os.Stat(fixture.output); !os.IsNotExist(err) {
					t.Fatalf("invalid evidence published history: %v", err)
				}
			})
		}
	}
}

// A main push without history must save its observed durations so the next
// plan can balance; that next, history-driven plan is enforced again.
func TestCISemaHistoryFallbackRefreshThenEnforces(t *testing.T) {
	root := t.TempDir()
	planner := filepath.Join(root, "cishard")
	if out, err := exec.Command(realGoCommand(t), "build", "-o", planner, "./internal/cishard").CombinedOutput(); err != nil {
		t.Fatalf("build planner: %v\n%s", err, out)
	}
	names := []string{"TestSemaA", "TestSemaB", "TestSemaC", "TestSemaD", "TestSemaE", "TestSemaF"}
	discovery := filepath.Join(root, "discovery.txt")
	if err := os.WriteFile(discovery, []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := func(history string) []byte {
		t.Helper()
		args := []string{"--package", semaFixturePackage, "--shards", "2", "--tests", discovery}
		if history != "" {
			args = append(args, "--history", history)
		}
		data, err := exec.Command(planner, args...).Output()
		if err != nil {
			t.Fatalf("planner: %v", err)
		}
		return data
	}
	refresh := func(event string, dirs []string, output string) (string, error) {
		t.Helper()
		cmd := exec.Command("bash", "ci-go-test.sh", "sema-history-refresh", dirs[0], dirs[1], output)
		cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME="+event)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	historyUsed := func(data []byte) bool {
		t.Helper()
		var parsed struct {
			HistoryUsed bool `json:"historyUsed"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		return parsed.HistoryUsed
	}

	// Run 1: no history, so the planner splits by count. Tests on shard 0 are
	// slow, giving a 6:1 skew.
	firstPlan := plan("")
	if historyUsed(firstPlan) {
		t.Fatalf("first plan must use the deterministic fallback: %s", firstPlan)
	}
	firstElapsed := func(shard int, _ string) float64 {
		if shard == 0 {
			return 30
		}
		return 5
	}
	first := writePlannedHistoryEvidence(t, filepath.Join(root, "run-1"), semaFixturePackage, names, firstPlan, firstElapsed)
	history := filepath.Join(root, "history", "sema-duration-history.json")
	out, err := refresh("push", first, history)
	if err != nil || !strings.Contains(out, "duration history timing advisory:") {
		t.Fatalf("fallback-planned push evidence must be saved with an advisory: %v\n%s", err, out)
	}
	if _, err := os.Stat(history); err != nil {
		t.Fatalf("fallback-planned push did not write history: %v", err)
	}

	// Run 2: the saved history drives a duration-balanced plan.
	secondPlan := plan(history)
	if !historyUsed(secondPlan) {
		t.Fatalf("second plan must use the saved history: %s", secondPlan)
	}
	var observed map[string]float64
	{
		observed = map[string]float64{}
		var parsed struct {
			Shards []struct {
				Tests []string `json:"tests"`
			} `json:"shards"`
		}
		if err := json.Unmarshal(firstPlan, &parsed); err != nil {
			t.Fatal(err)
		}
		for shard, entry := range parsed.Shards {
			for _, name := range entry.Tests {
				observed[name] = firstElapsed(shard, name)
			}
		}
	}
	steady := writePlannedHistoryEvidence(t, filepath.Join(root, "run-2-steady"), semaFixturePackage, names, secondPlan,
		func(_ int, name string) float64 { return observed[name] })
	steadyHistory := filepath.Join(root, "steady", "sema-duration-history.json")
	if out, err := refresh("push", steady, steadyHistory); err != nil || strings.Contains(out, "timing advisory:") {
		t.Fatalf("history-planned balanced evidence must refresh without an advisory: %v\n%s", err, out)
	}

	// The same history-driven plan with a slow shard 0 is rejected on push, so
	// the advisory never becomes permanent.
	skewed := writePlannedHistoryEvidence(t, filepath.Join(root, "run-2-skewed"), semaFixturePackage, names, secondPlan,
		func(shard int, name string) float64 {
			if shard == 0 {
				return observed[name] * 4
			}
			return observed[name]
		})
	skewedHistory := filepath.Join(root, "skewed", "sema-duration-history.json")
	out, err = refresh("push", skewed, skewedHistory)
	if err == nil || !strings.Contains(out, "exceeds 1.5x 2-shard median") || strings.Contains(out, "timing advisory:") {
		t.Fatalf("history-planned imbalance must be rejected on push: %v\n%s", err, out)
	}
	if _, err := os.Stat(skewedHistory); !os.IsNotExist(err) {
		t.Fatalf("rejected refresh left history: %v", err)
	}

	// Pull requests keep their advisory for the same evidence.
	out, err = refresh("pull_request", skewed, skewedHistory)
	if err != nil || !strings.Contains(out, "duration history timing advisory:") {
		t.Fatalf("PR timing must stay advisory: %v\n%s", err, out)
	}
}

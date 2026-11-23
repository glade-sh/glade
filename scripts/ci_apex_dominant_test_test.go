package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// multiTestHistoryFixture writes one shard directory per entry of shardTests.
// Each entry lists the Elapsed seconds of that shard's tests; the shard owns
// TestDom<shard>x<position>. The plan used duration history.
func multiTestHistoryFixture(t *testing.T, packageName string, shardTests [][]float64) (string, []string) {
	t.Helper()
	root := t.TempDir()
	var all []string
	shards := make([]map[string]any, len(shardTests))
	names := make([][]string, len(shardTests))
	for i, durations := range shardTests {
		for j := range durations {
			names[i] = append(names[i], fmt.Sprintf("TestDom%02dx%02d", i, j))
		}
		all = append(all, names[i]...)
		shards[i] = map[string]any{"index": i, "tests": names[i], "estimatedDurationMillis": 0,
			"regex": "^(?:" + strings.Join(names[i], "|") + ")$"}
	}
	plan := map[string]any{"version": 1, "package": packageName, "historyUsed": true, "shards": shards}
	dirs := make([]string, len(shardTests))
	for i := range dirs {
		dirs[i] = filepath.Join(root, fmt.Sprint(i))
		if err := os.Mkdir(dirs[i], 0700); err != nil {
			t.Fatal(err)
		}
		writeJSONFixture(t, filepath.Join(dirs[i], "plan.json"), plan)
		writeJSONFixture(t, filepath.Join(dirs[i], "selected-shard.json"), shards[i])
		writeJSONFixture(t, filepath.Join(dirs[i], "validation-summary.json"), map[string]any{
			"valid": true, "expected": names[i], "passed": names[i], "errors": []string{},
		})
		var events strings.Builder
		for j, seconds := range shardTests[i] {
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%g}\n", packageName, names[i][j], seconds)
		}
		fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q}\n", packageName)
		for file, data := range map[string]string{
			"discovery.txt": strings.Join(all, "\n") + "\n",
			"events.json":   events.String(),
		} {
			if err := os.WriteFile(filepath.Join(dirs[i], file), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return filepath.Join(root, "history.json"), dirs
}

func runMultiTestHistory(t *testing.T, packageName, output string, dirs []string) (string, error) {
	t.Helper()
	args := append([]string{"-c", `source "$1"; shift; refresh_test_history "$@"`, "history", "ci-go-test.sh",
		packageName, "Apex", "apextest", "0", fmt.Sprint(len(dirs)), output}, dirs...)
	out, err := exec.Command("bash", args...).CombinedOutput()
	return string(out), err
}

// A shard over 1.5x the median because of one indivisible test keeps its
// history only when it is within 1.5x the median without that test, within
// 2.5x the median as a whole, and that test is not over twice its own recorded
// duration. Every other imbalance rejects.
func TestCIApexHistoryBalanceGateToleratesOneDominantTest(t *testing.T) {
	// Fixed other shards hold the four-shard median at 100s: the ratio limit
	// is 150s and the whole-shard cap is 250s.
	hundred := [][]float64{{50, 50}, {50, 50}, {50, 50}}
	cases := []struct {
		name     string
		shard0   []float64
		others   [][]float64
		recorded map[string]float64 // seconds, written as the predecessor history
		accept   bool
	}{
		// The two observed failures, scaled to their own medians.
		{"run 43921954853 E=1023 L=364 M=671", []float64{364, 110, 110, 110, 110, 110, 109}, [][]float64{{335.5, 335.5}, {335.5, 335.5}, {335.5, 335.5}}, map[string]float64{"TestDom00x00": 221}, true},
		{"run 43881372471 E=971 L=591 M=646", []float64{591, 380}, [][]float64{{323, 323}, {323, 323}, {323, 323}}, map[string]float64{"TestDom00x00": 378}, true},
		{"largest test 35 percent and the remainder at the ratio", []float64{80, 50, 50, 50}, hundred, nil, true},
		{"largest test at the ratio boundary", []float64{100, 50, 50}, hundred, nil, true},
		{"just over the whole-shard cap", []float64{100, 100, 50.001}, hundred, nil, false},
		// Real imbalance.
		{"hair over the remainder bound", []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 0.001}, hundred, nil, false},
		{"largest test 3x the median with the shard over 2.5x", []float64{300, 100}, hundred, nil, false},
		{"one test far above the median", []float64{600, 100, 100, 100}, hundred, nil, false},
		{"largest test at 60 percent with a remainder over the ratio", []float64{300, 200}, hundred, nil, false},
		{"largest test 35 percent and the remainder over the ratio", []float64{364, 200, 160, 160, 160}, hundred, nil, false},
		{"many equal tests all slow", []float64{50, 50, 50, 50, 50, 50}, hundred, nil, false},
		{"largest test below 60 percent and remainder over the ratio", []float64{80, 90, 90}, hundred, nil, false},
		// One test regressed against its own record.
		{"largest test at 2x its record", []float64{100, 50, 50}, hundred, map[string]float64{"TestDom00x00": 50}, true},
		{"largest test over 2x its record", []float64{100, 50, 50}, hundred, map[string]float64{"TestDom00x00": 49}, false},
		{"largest test over 2x its record with no other skew", []float64{200, 40}, hundred, map[string]float64{"TestDom00x00": 90}, false},
		{"largest test with no record", []float64{200, 40}, hundred, nil, true},
	}
	for _, event := range []string{"push", "schedule", "workflow_dispatch"} {
		for _, tc := range cases {
			t.Run(event+"/"+tc.name, func(t *testing.T) {
				t.Setenv("GITHUB_EVENT_NAME", event)
				t.Setenv("GITHUB_HEAD_REF", "")
				t.Setenv("CI_APEXTEST_HISTORY_PATH", "")
				if tc.recorded != nil {
					var rows []map[string]any
					for name, seconds := range tc.recorded {
						rows = append(rows, map[string]any{"name": name, "durationMillis": int64(seconds * 1000)})
					}
					path := filepath.Join(t.TempDir(), "recorded.json")
					writeJSONFixture(t, path, map[string]any{"version": 1, "package": fixturePackage, "complete": true, "tests": rows})
					t.Setenv("CI_APEXTEST_HISTORY_PATH", path)
				}
				output, dirs := multiTestHistoryFixture(t, fixturePackage, append([][]float64{tc.shard0}, tc.others...))
				out, err := runMultiTestHistory(t, fixturePackage, output, dirs)
				if tc.accept {
					if err != nil || !strings.Contains(out, "duration history timing advisory:") || !strings.Contains(out, "dominated by one test") {
						t.Fatalf("one-test overrun must keep history with an advisory: %v\n%s", err, out)
					}
					if got := readWallHistory(t, output); len(got) != len(tc.shard0)+2*len(tc.others) {
						t.Fatalf("history lost tests: %v", got)
					}
					return
				}
				if err == nil || !strings.Contains(out, "exceeds 1.5x 4-shard median") || strings.Contains(out, "timing advisory:") {
					t.Fatalf("real imbalance must be rejected: %v\n%s", err, out)
				}
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatalf("rejected refresh left history: %v", err)
				}
			})
		}
	}
}

// The tolerance is Apex only; the sema balance gate is unchanged.
func TestCISemaBalanceGateKeepsRejectingOneDominantTest(t *testing.T) {
	t.Setenv("GITHUB_EVENT_NAME", "push")
	t.Setenv("GITHUB_HEAD_REF", "")
	output, dirs := multiTestHistoryFixture(t, semaFixturePackage, [][]float64{{400, 100, 100}, {50, 50}, {50, 50}, {50, 50}})
	out, err := runMultiTestHistory(t, semaFixturePackage, output, dirs)
	if err == nil || !strings.Contains(out, "exceeds 1.5x 4-shard median") {
		t.Fatalf("sema must still reject: %v\n%s", err, out)
	}
}

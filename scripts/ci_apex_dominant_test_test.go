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
// history. Imbalance that survives removing that test still rejects.
func TestCIApexHistoryBalanceGateToleratesOneDominantTest(t *testing.T) {
	// Three other shards of 100s put the four-shard median at 100s and the
	// limit at 150s.
	others := func(first []float64) [][]float64 {
		return [][]float64{first, {50, 50}, {50, 50}, {50, 50}}
	}
	cases := []struct {
		name   string
		shard0 []float64
		accept bool
	}{
		{"one test holds most of the shard", []float64{400, 100, 100}, true},
		{"largest test at exactly 60 percent", []float64{300, 200}, true},
		{"one test above 60 percent with several small tests", []float64{600, 100, 100, 100}, true},
		// The release run shape: the largest test is about 35% of the shard
		// and the other tests are slow but would fit without it.
		{"largest test 35 percent and the remainder within the ratio", []float64{80, 50, 50, 50}, true},
		{"largest test 35 percent and the remainder over the ratio", []float64{364, 200, 160, 160, 160}, false},
		{"many equal tests all slow", []float64{50, 50, 50, 50, 50, 50}, false},
		{"largest test below 60 percent and remainder over the ratio", []float64{80, 90, 90}, false},
		{"largest test just under 60 percent with a heavy remainder", []float64{290, 200}, false},
	}
	for _, event := range []string{"push", "schedule", "workflow_dispatch"} {
		for _, tc := range cases {
			t.Run(event+"/"+tc.name, func(t *testing.T) {
				t.Setenv("GITHUB_EVENT_NAME", event)
				t.Setenv("GITHUB_HEAD_REF", "")
				output, dirs := multiTestHistoryFixture(t, fixturePackage, others(tc.shard0))
				out, err := runMultiTestHistory(t, fixturePackage, output, dirs)
				if tc.accept {
					if err != nil || !strings.Contains(out, "duration history timing advisory:") || !strings.Contains(out, "dominated by one test") {
						t.Fatalf("one-test overrun must keep history with an advisory: %v\n%s", err, out)
					}
					if got := readWallHistory(t, output); len(got) != len(tc.shard0)+6 {
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

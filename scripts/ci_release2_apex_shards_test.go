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

func TestCIRelease2ApexTwelveShardRoutesAndHistory(t *testing.T) {
	testCIRelease2ApexShardRoutesAndHistory(t, 12)
}

func TestCIRelease2ApexFourteenShardRoutesAndHistory(t *testing.T) {
	testCIRelease2ApexShardRoutesAndHistory(t, 14)
}

func testCIRelease2ApexShardRoutesAndHistory(t *testing.T, count int) {
	t.Helper()
	t.Setenv("CI_APEXTEST_SHARDS", fmt.Sprint(count))
	names := make([]string, count)
	shards := make([]map[string]any, count)
	var discovery strings.Builder
	for index := range names {
		name := fmt.Sprintf("TestScale%02d", index)
		names[index] = name
		fmt.Fprintln(&discovery, name)
		shards[index] = map[string]any{"index": index, "tests": []string{name}, "estimatedDurationMillis": 1000, "regex": "^(?:" + name + ")$"}
	}
	plan, err := json.Marshal(map[string]any{"version": 1, "package": fixturePackage, "historyUsed": true, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, count)
	for index, name := range names {
		out, err, artifacts := runApexShardFixture(t, fmt.Sprint(index), discovery.String(), 0, string(plan), 0, passEvent(name), 0)
		if err != nil {
			t.Fatalf("%d-shard dispatcher %d failed: %v\n%s", count, index, err, out)
		}
		dirs[index] = artifacts
	}
	output := filepath.Join(t.TempDir(), "history.json")
	cmd := exec.Command("bash", append([]string{"ci-go-test.sh", "apex-history-refresh", output}, dirs...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%d-shard history failed: %v\n%s", count, err, out)
	}
	assertRelease2HistoryNames(t, output, names)
}

func assertRelease2HistoryNames(t *testing.T, path string, want []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Complete bool `json:"complete"`
		Tests    []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	if !history.Complete || len(history.Tests) != len(want) {
		t.Fatalf("incomplete history: %s", data)
	}
	for index, name := range want {
		if history.Tests[index].Name != name || history.Tests[index].DurationMillis != 1000 {
			t.Fatalf("unexpected history row %d: %s", index, data)
		}
	}
}

func TestCIRelease2ApexTwelveShardRejectsInvalidIndices(t *testing.T) {
	for _, index := range []string{"12", "-1", "00"} {
		t.Run(index, func(t *testing.T) {
			cmd := exec.Command("bash", "ci-go-test.sh", "apex-shard", index)
			cmd.Env = append(os.Environ(), "CI_APEXTEST_SHARDS=12", "CI_APEXTEST_ARTIFACT_DIR="+t.TempDir())
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "shard index must be between 0 and 11") {
				t.Fatalf("invalid shard %q: %v\n%s", index, err, out)
			}
		})
	}
}

func TestCIRelease2ApexFourteenShardRejectsInvalidIndices(t *testing.T) {
	for _, index := range []string{"14", "-1", "00"} {
		t.Run(index, func(t *testing.T) {
			cmd := exec.Command("bash", "ci-go-test.sh", "apex-shard", index)
			cmd.Env = append(os.Environ(), "CI_APEXTEST_SHARDS=14", "CI_APEXTEST_ARTIFACT_DIR="+t.TempDir())
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "shard index must be between 0 and 13") {
				t.Fatalf("invalid shard %q: %v\n%s", index, err, out)
			}
		})
	}
}

func TestCIRelease2PackageShardCountsRejectOutOfRangeIndices(t *testing.T) {
	for _, tc := range []struct{ route, env, count, index, want string }{
		{"vm-shard", "CI_VM_SHARDS", "3", "3", "between 0 and 2"},
		{"gladecli-shard", "CI_GLADECLI_SHARDS", "2", "2", "0 or 1"},
		{"vm-shard", "CI_VM_SHARDS", "4", "4", "between 0 and 3"},
		{"gladecli-shard", "CI_GLADECLI_SHARDS", "4", "4", "between 0 and 3"},
	} {
		t.Run(tc.route+"-"+tc.count+"-"+tc.index, func(t *testing.T) {
			cmd := exec.Command("bash", "ci-go-test.sh", tc.route, tc.index)
			cmd.Env = append(os.Environ(), tc.env+"="+tc.count, "CI_VM_ARTIFACT_DIR="+t.TempDir(), "CI_GLADECLI_ARTIFACT_DIR="+t.TempDir())
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.want) {
				t.Fatalf("%s index %s of %s: %v\n%s", tc.route, tc.index, tc.count, err, out)
			}
		})
	}
}

func TestCIRelease2PackageShardCountsRejectUnsupportedCounts(t *testing.T) {
	for _, tc := range []struct{ route, env, count string }{{"vm-shard", "CI_VM_SHARDS", "2"}, {"gladecli-shard", "CI_GLADECLI_SHARDS", "3"}} {
		cmd := exec.Command("bash", "ci-go-test.sh", tc.route, "0")
		cmd.Env = append(os.Environ(), tc.env+"="+tc.count)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), tc.env+" must be") {
			t.Fatalf("%s=%s: %v\n%s", tc.env, tc.count, err, out)
		}
	}
}

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

const case319Package = "github.com/glade-sh/glade/internal/apextest"

type case319BehaviorFixture struct{ root, evidence, profile, history, stage, bin string }

func newCase319BehaviorFixture(t *testing.T) case319BehaviorFixture {
	t.Helper()
	root := t.TempDir()
	f := case319BehaviorFixture{root: root, evidence: filepath.Join(root, "evidence"), profile: filepath.Join(root, "profile.json"), history: filepath.Join(root, "history", "apextest-duration-history.json"), stage: filepath.Join(root, "stage"), bin: filepath.Join(root, "bin")}
	names := make([]string, 12)
	shards := make([]map[string]any, 12)
	for i := range names {
		names[i] = fmt.Sprintf("TestBootstrap%02d", i)
		shards[i] = map[string]any{"index": i, "tests": []string{names[i]}, "estimatedDurationMillis": 0, "regex": "^(?:" + names[i] + ")$"}
	}
	plan, err := json.Marshal(map[string]any{"version": 1, "package": case319Package, "historyUsed": false, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	dirs := writePlannedHistoryEvidence(t, f.evidence, case319Package, names, plan, func(i int, _ string) float64 { return float64(i + 1) })
	for i, dir := range dirs {
		target := filepath.Join(f.evidence, fmt.Sprintf("apex-shard-%d", i))
		if err := os.Rename(dir, target); err != nil {
			t.Fatal(err)
		}
		dirs[i] = target
	}
	// Verify the same twelve real refresh inputs before testing the workflow route.
	args := append([]string{"ci-go-test.sh", "apex-history-refresh", filepath.Join(root, "verified.json")}, dirs...)
	verify := exec.Command("bash", args...)
	verify.Env = append(os.Environ(), "GITHUB_EVENT_NAME=push")
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("invalid predecessor fixture: %v\n%s", err, out)
	}
	if err := os.MkdirAll(f.bin, 0700); err != nil {
		t.Fatal(err)
	}
	gh := `#!/bin/bash
set -eu
if [ "$1" = api ]; then echo 901; exit 0; fi
[ "$1" = run ] && [ "$2" = download ] || exit 2
shift 3
name=""; dest=""
while [ "$#" -gt 0 ]; do case "$1" in --name) name=$2; shift 2;; --pattern) shift 2;; --dir) dest=$2; shift 2;; *) exit 2;; esac; done
mkdir -p "$dest"
if [ -n "$name" ]; then [ "$name" = apextest-scheduling-profile-777-1 ] || exit 2; cp "$FAKE_PROFILE" "$dest/profile.json"; else cp -R "$FAKE_EVIDENCE"/apex-shard-* "$dest/"; fi
`
	if err := os.WriteFile(filepath.Join(f.bin, "gh"), []byte(gh), 0700); err != nil {
		t.Fatal(err)
	}
	return f
}

// Run the actual input route of whichever tree is checked out. The original
// workflow has an inline staging step; the corrected workflow invokes a helper.
func runCase319BehaviorRoute(t *testing.T, f case319BehaviorFixture, attempt string) map[string]int64 {
	t.Helper()
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	start := strings.Index(workflow, "  apextest-history-input:\n")
	end := strings.Index(workflow, "\n  apextest:\n")
	if start < 0 || end <= start {
		t.Fatal("missing input job")
	}
	job := workflow[start:end]
	stage := f.stage
	var cmd *exec.Cmd
	if strings.Contains(job, "scripts/ci-apex-history-input.sh") {
		cmd = exec.Command("bash", "ci-apex-history-input.sh")
	} else {
		marker := "      - name: Stage Apex duration history\n        run: |\n"
		index := strings.Index(job, marker)
		if index < 0 {
			t.Fatal("missing inline stage")
		}
		var lines []string
		for _, line := range strings.Split(job[index+len(marker):], "\n") {
			if !strings.HasPrefix(line, "          ") {
				break
			}
			lines = append(lines, strings.TrimPrefix(line, "          "))
		}
		cmd = exec.Command("bash", "-c", strings.Join(lines, "\n"))
		cmd.Dir = f.root
		stage = filepath.Join(f.root, "ci-artifacts", "apextest-history-input")
	}
	cmd.Env = append(os.Environ(), "PATH="+f.bin+":"+os.Getenv("PATH"), "FAKE_EVIDENCE="+f.evidence, "FAKE_PROFILE="+f.profile, "GITHUB_REPOSITORY=example/repo", "GITHUB_RUN_ID=777", "GITHUB_RUN_ATTEMPT="+attempt, "GITHUB_EVENT_NAME=push", "CI_DURATION_HISTORY_CACHE_REF=abc", "CI_APEXTEST_HISTORY_PATH="+f.history, "CI_APEXTEST_HISTORY_STAGE="+stage, "CI_APEXTEST_BENCHMARK_PAIR=0", "CI_APEXTEST_RETRY_PROFILE_PATH="+filepath.Join(f.root, "no-retry-cache.json"), "RUNNER_TEMP="+f.root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("workflow input route failed: %v\n%s", err, out)
	}
	data, err = os.ReadFile(filepath.Join(stage, "apextest-duration-history.json"))
	if os.IsNotExist(err) {
		return map[string]int64{}
	}
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		Tests []struct {
			Name     string `json:"name"`
			Duration int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	weights := map[string]int64{}
	for _, row := range profile.Tests {
		weights[row.Name] = row.Duration
	}
	return weights
}

func TestCICase319BehavioralBootstrapOriginal(t *testing.T) {
	f := newCase319BehaviorFixture(t)
	weights := runCase319BehaviorRoute(t, f, "1")
	if got := weights["TestBootstrap11"]; got != 12000 {
		t.Fatalf("workflow staged predecessor duration %d ms, want 12000 ms", got)
	}
}
func TestCICase319BehavioralRetryOriginal(t *testing.T) {
	f := newCase319BehaviorFixture(t)
	if err := os.MkdirAll(filepath.Dir(f.history), 0700); err != nil {
		t.Fatal(err)
	}
	stale := fmt.Sprintf(`{"version":1,"package":%q,"complete":true,"tests":[{"name":"TestBootstrap00","durationMillis":1}]}`, case319Package)
	fresh := fmt.Sprintf(`{"version":1,"package":%q,"complete":true,"tests":[{"name":"TestBootstrap00","durationMillis":42000}]}`, case319Package)
	if err := os.WriteFile(f.history, []byte(stale), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.profile, []byte(fresh), 0600); err != nil {
		t.Fatal(err)
	}
	weights := runCase319BehaviorRoute(t, f, "2")
	if got := weights["TestBootstrap00"]; got != 42000 {
		t.Fatalf("retry staged stale duration %d ms, want 42000 ms", got)
	}
}

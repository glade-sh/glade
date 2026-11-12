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

// This executes the history-input route as it existed in either tree. On the
// old tree the staging step is inline; on the current tree it calls the helper.
func TestCICase319ConnectedBootstrapRegression(t *testing.T) {
	root := t.TempDir()
	names := make([]string, 12)
	shards := make([]map[string]any, 12)
	for i := range names {
		names[i] = fmt.Sprintf("TestBootstrap%02d", i)
		shards[i] = map[string]any{"index": i, "tests": []string{names[i]}, "estimatedDurationMillis": 0, "regex": "^(?:" + names[i] + ")$"}
	}
	plan, err := json.Marshal(map[string]any{"version": 1, "package": "github.com/glade-sh/glade/internal/apextest", "historyUsed": false, "shards": shards})
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, "evidence")
	dirs := writePlannedHistoryEvidence(t, evidence, "github.com/glade-sh/glade/internal/apextest", names, plan, func(i int, _ string) float64 { return float64(i + 1) })
	for i, dir := range dirs {
		if err := os.Rename(dir, filepath.Join(evidence, fmt.Sprintf("apex-shard-%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	fake := `#!/bin/bash
if [ "$1" = api ]; then echo 901; else mkdir -p "$7"; cp -R "$FAKE_EVIDENCE"/apex-shard-* "$7/"; fi
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fake), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	begin := strings.Index(workflow, "  history-input:\n")
	end := strings.Index(workflow, "\n  gladecli:\n")
	if begin < 0 || end <= begin {
		t.Fatal("missing workflow job")
	}
	job := workflow[begin:end]
	var cmd *exec.Cmd
	stage := filepath.Join(root, "stage")
	if strings.Contains(job, "scripts/ci-apex-history-input.sh") {
		cmd = exec.Command("bash", "ci-apex-history-input.sh")
	} else {
		marker := "      - name: Stage Apex duration history\n        run: |\n"
		start := strings.Index(job, marker)
		if start < 0 {
			t.Fatal("missing original stage step")
		}
		rest := job[start+len(marker):]
		var lines []string
		for _, line := range strings.Split(rest, "\n") {
			if !strings.HasPrefix(line, "          ") {
				break
			}
			lines = append(lines, strings.TrimPrefix(line, "          "))
		}
		cmd = exec.Command("bash", "-c", strings.Join(lines, "\n"))
		cmd.Dir = root
		stage = filepath.Join(root, "ci-artifacts", "apextest-history-input")
	}
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "FAKE_EVIDENCE="+evidence, "GITHUB_REPOSITORY=example/repo", "GITHUB_RUN_ID=777", "GITHUB_RUN_ATTEMPT=1", "GITHUB_EVENT_NAME=push", "CI_DURATION_HISTORY_CACHE_REF=abc", "CI_APEXTEST_HISTORY_PATH="+filepath.Join(root, "history", "apextest-duration-history.json"), "CI_APEXTEST_HISTORY_STAGE="+stage, "CI_APEXTEST_BENCHMARK_PAIR=0", "RUNNER_TEMP="+root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("workflow input route: %v\n%s", err, out)
	}
	history, err := os.ReadFile(filepath.Join(stage, "apextest-duration-history.json"))
	if err != nil {
		t.Fatalf("valid predecessor was not staged: %v", err)
	}
	if !strings.Contains(string(history), `"durationMillis": 12000`) {
		t.Fatalf("predecessor weight missing: %s", history)
	}
}

func TestCICase319RetryUsesCachedSchedulingProfile(t *testing.T) {
	root := t.TempDir()
	pkg := "github.com/glade-sh/glade/internal/apextest"
	stale := fmt.Sprintf(`{"version":1,"package":%q,"complete":true,"tests":[{"name":"TestA","durationMillis":1}]}`, pkg)
	fresh := fmt.Sprintf(`{"version":1,"package":%q,"complete":true,"tests":[{"name":"TestA","durationMillis":42000}]}`, pkg)
	history := filepath.Join(root, "history.json")
	profile := filepath.Join(root, "cached-profile.json")
	if err := os.WriteFile(history, []byte(stale), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte(fresh), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/bash\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(root, "stage")
	cmd := exec.Command("bash", "ci-apex-history-input.sh")
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "GITHUB_REPOSITORY=example/repo", "GITHUB_RUN_ID=777", "GITHUB_RUN_ATTEMPT=2", "GITHUB_EVENT_NAME=push", "CI_DURATION_HISTORY_CACHE_REF=abc", "CI_APEXTEST_HISTORY_PATH="+history, "CI_APEXTEST_HISTORY_STAGE="+stage, "CI_APEXTEST_BENCHMARK_PAIR=0", "CI_APEXTEST_RETRY_PROFILE_PATH="+profile, "RUNNER_TEMP="+root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("retry input: %v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(stage, "apextest-duration-history.json"))
	if err != nil || string(got) != fresh {
		t.Fatalf("cached profile not used when artifact unavailable: %s %v", got, err)
	}
	workflow, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Restore Apex retry scheduling profile", "Cache Apex scheduling profile after balance failure", "apextest-scheduling-v1-"} {
		if !strings.Contains(string(workflow), want) {
			t.Errorf("workflow missing %q", want)
		}
	}
}

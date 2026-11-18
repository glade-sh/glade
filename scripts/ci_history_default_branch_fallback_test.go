package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Shard duration history normally comes from the run for the predecessor
// commit. When that lookup finds nothing, the newest successful default-branch
// push run seeds the history instead. These tests pin that fallback for the
// Apex script and for the LWC and Visualforce workflow steps.

const historyFallbackApexGH = `#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$FAKE_CALLS"
if [ "$1" = api ]; then
  case "$*" in
    *head_sha=*) printf '%s' "${FAKE_REF_RUNS:-}";;
    *branch=*) printf '%s' "${FAKE_BRANCH_RUNS:-}";;
    *) exit 2;;
  esac
  exit 0
fi
[ "$1" = run ] && [ "$2" = download ] || exit 2
printf '%s\n' "$3" >> "$FAKE_DOWNLOADS"
shift 3
dest=""
while [ "$#" -gt 0 ]; do case "$1" in --pattern) shift 2;; --dir) dest=$2; shift 2;; *) exit 2;; esac; done
mkdir -p "$dest"
cp -R "$FAKE_EVIDENCE"/apex-shard-* "$dest/"
`

type historyFallbackApexResult struct {
	calls, downloads, output string
	staged                   bool
}

func runHistoryFallbackApex(t *testing.T, refRuns, branchRuns, fallbackBranch, pair string) historyFallbackApexResult {
	t.Helper()
	f := newCase319BehaviorFixture(t)
	writeApexFixtureExecutable(t, filepath.Join(f.bin, "gh"), historyFallbackApexGH)
	calls := filepath.Join(f.root, "calls")
	downloads := filepath.Join(f.root, "downloads")
	for _, p := range []string{calls, downloads} {
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", "ci-apex-history-input.sh")
	cmd.Env = append(os.Environ(), "PATH="+f.bin+":"+os.Getenv("PATH"),
		"FAKE_CALLS="+calls, "FAKE_DOWNLOADS="+downloads, "FAKE_EVIDENCE="+f.evidence,
		"FAKE_REF_RUNS="+refRuns, "FAKE_BRANCH_RUNS="+branchRuns,
		"GITHUB_REPOSITORY=example/repo", "GITHUB_RUN_ID=777", "GITHUB_RUN_ATTEMPT=1", "GITHUB_EVENT_NAME=workflow_dispatch",
		"CI_DURATION_HISTORY_CACHE_REF=abc", "CI_DURATION_HISTORY_FALLBACK_BRANCH="+fallbackBranch,
		"CI_APEXTEST_HISTORY_PATH="+f.history, "CI_APEXTEST_HISTORY_STAGE="+f.stage,
		"CI_APEXTEST_BENCHMARK_PAIR="+pair,
		"CI_APEXTEST_RETRY_PROFILE_PATH="+filepath.Join(f.root, "no-retry-cache.json"), "RUNNER_TEMP="+f.root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("history input script failed: %v\n%s", err, out)
	}
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	_, statErr := os.Stat(filepath.Join(f.stage, "apextest-duration-history.json"))
	return historyFallbackApexResult{calls: read(calls), downloads: read(downloads), output: string(out), staged: statErr == nil}
}

func TestCIHistoryDefaultBranchFallbackApexUsesDefaultBranchRun(t *testing.T) {
	r := runHistoryFallbackApex(t, "", "955\n", "main", "0")
	if !strings.Contains(r.calls, "branch=main") || !strings.Contains(r.calls, "event=push") {
		t.Fatalf("default-branch query missing:\n%s", r.calls)
	}
	if r.downloads != "955\n" || !r.staged {
		t.Fatalf("default-branch evidence not staged: downloads=%q staged=%v\n%s", r.downloads, r.staged, r.output)
	}
}

func TestCIHistoryDefaultBranchFallbackApexUnsetKeepsBehavior(t *testing.T) {
	r := runHistoryFallbackApex(t, "", "955\n", "", "0")
	if strings.Contains(r.calls, "branch=") || r.downloads != "" || r.staged {
		t.Fatalf("unset fallback changed behavior: calls=%q downloads=%q staged=%v", r.calls, r.downloads, r.staged)
	}
	if !strings.Contains(r.output, "using deterministic scheduling fallback") {
		t.Fatalf("missing deterministic fallback warning:\n%s", r.output)
	}
}

func TestCIHistoryDefaultBranchFallbackApexSkippedWhenRefHistoryUsable(t *testing.T) {
	r := runHistoryFallbackApex(t, "901\n", "955\n", "main", "0")
	if strings.Contains(r.calls, "branch=") || r.downloads != "901\n" || !r.staged {
		t.Fatalf("usable predecessor history must not query default branch: calls=%q downloads=%q staged=%v", r.calls, r.downloads, r.staged)
	}
}

func TestCIHistoryDefaultBranchFallbackApexNothingFoundKeepsDeterministic(t *testing.T) {
	r := runHistoryFallbackApex(t, "", "", "main", "0")
	if !strings.Contains(r.calls, "branch=main") || r.staged || !strings.Contains(r.output, "using deterministic scheduling fallback") {
		t.Fatalf("empty fallback must keep deterministic scheduling: calls=%q staged=%v\n%s", r.calls, r.staged, r.output)
	}
}

func TestCIHistoryDefaultBranchFallbackApexBenchmarkPairMakesNoLookups(t *testing.T) {
	r := runHistoryFallbackApex(t, "901\n", "955\n", "main", "3")
	if r.calls != "" || r.downloads != "" || r.staged {
		t.Fatalf("benchmark pair must not look up predecessors: calls=%q downloads=%q staged=%v", r.calls, r.downloads, r.staged)
	}
}

type historyFallbackStep struct {
	Name string
	Env  map[string]string
	Run  string
}

func historyFallbackWorkflowStep(t *testing.T, job, name string) historyFallbackStep {
	t.Helper()
	var workflow struct {
		Jobs map[string]struct {
			Steps []historyFallbackStep
		}
	}
	if err := yaml.Unmarshal([]byte(readWorkflowFile(t, "ci.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, step := range workflow.Jobs[job].Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("job %s has no step %q", job, name)
	return historyFallbackStep{}
}

const historyFallbackStepGH = `#!/bin/bash
printf '%s\n' "$*" >> "$FAKE_CALLS"
[ "$1" = api ] || exit 2
case "$*" in
  *head_sha=*) printf '%s' "${FAKE_REF_ID:-}";;
  *branch=*) [ -z "${FAKE_BRANCH_FAIL:-}" ] || exit 1; printf '%s' "${FAKE_BRANCH_ID:-}";;
  */artifacts*) printf 'go-test-visualforce-shard-0\ngo-test-visualforce-shard-1\ngo-test-visualforce-shard-2\ngo-test-visualforce-shard-3\n';;
  *) exit 2;;
esac
`

func runHistoryFallbackStep(t *testing.T, job, name, historyVar, branchID string, branchFail bool) (output, calls, stdout string) {
	t.Helper()
	step := historyFallbackWorkflowStep(t, job, name)
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeApexFixtureExecutable(t, filepath.Join(bin, "gh"), historyFallbackStepGH)
	callsPath := filepath.Join(root, "calls")
	outPath := filepath.Join(root, "output")
	for _, p := range []string{callsPath, outPath} {
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"),
		"FAKE_CALLS="+callsPath, "FAKE_BRANCH_ID="+branchID,
		"GITHUB_REPOSITORY=example/repo", "GITHUB_OUTPUT="+outPath,
		"CI_DURATION_HISTORY_CACHE_REF=abc", historyVar+"="+filepath.Join(root, "absent.json"),
		"DEFAULT_BRANCH=main")
	if branchFail {
		env = append(env, "FAKE_BRANCH_FAIL=1")
	}
	cmd := exec.Command("bash", "-eo", "pipefail", "-c", step.Run)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("step must not fail: %v\n%s", err, out)
	}
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return string(out), read(callsPath), read(outPath)
}

func TestCIHistoryDefaultBranchFallbackWorkflowSteps(t *testing.T) {
	cases := []struct {
		job, step, historyVar string
		extra                 string
	}{
		{"history-input", "Find predecessor LWC shard evidence", "CI_LWC_COMPILE_HISTORY_PATH", ""},
		{"history-input", "Find predecessor Visualforce evidence", "CI_VISUALFORCE_HISTORY_PATH", "format=shards\n"},
	}
	for _, c := range cases {
		t.Run(c.step+"/default-branch-run-used", func(t *testing.T) {
			_, calls, out := runHistoryFallbackStep(t, c.job, c.step, c.historyVar, "4242", false)
			if !strings.Contains(calls, "head_sha=abc") || !strings.Contains(calls, "branch=main") || !strings.Contains(calls, "event=push") {
				t.Fatalf("expected both lookups:\n%s", calls)
			}
			if !strings.Contains(out, "run_id=4242\n") || !strings.Contains(out, c.extra) {
				t.Fatalf("GITHUB_OUTPUT = %q, want run_id=4242 and %q", out, c.extra)
			}
		})
		t.Run(c.step+"/both-empty", func(t *testing.T) {
			_, calls, out := runHistoryFallbackStep(t, c.job, c.step, c.historyVar, "", false)
			if !strings.Contains(calls, "branch=main") {
				t.Fatalf("fallback lookup not attempted:\n%s", calls)
			}
			if strings.Contains(out, "run_id=4242") || strings.Contains(out, "format=") {
				t.Fatalf("unexpected outputs: %q", out)
			}
			if c.extra == "" && out != "run_id=\n" && out != "" {
				t.Fatalf("unexpected LWC output %q", out)
			}
		})
		t.Run(c.step+"/fallback-lookup-fails", func(t *testing.T) {
			out, _, outFile := runHistoryFallbackStep(t, c.job, c.step, c.historyVar, "4242", true)
			if !strings.Contains(out, "::warning::") {
				t.Fatalf("missing warning:\n%s", out)
			}
			if strings.Contains(outFile, "run_id=4242") {
				t.Fatalf("failed lookup produced a run id: %q", outFile)
			}
		})
	}
}

func TestCIHistoryDefaultBranchFallbackEnvWiring(t *testing.T) {
	const want = "${{ github.event.repository.default_branch }}"
	for _, c := range []struct{ job, step, key string }{
		{"history-input", "Find predecessor LWC shard evidence", "DEFAULT_BRANCH"},
		{"history-input", "Find predecessor Visualforce evidence", "DEFAULT_BRANCH"},
		{"history-input", "Stage validated Apex scheduling input", "CI_DURATION_HISTORY_FALLBACK_BRANCH"},
	} {
		step := historyFallbackWorkflowStep(t, c.job, c.step)
		if step.Env[c.key] != want {
			t.Errorf("%s / %s: env %s = %q, want %q", c.job, c.step, c.key, step.Env[c.key], want)
		}
	}
}

package scripts

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type prLaneStep struct {
	Name string            `yaml:"name"`
	If   string            `yaml:"if"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
}

type prLaneJob struct {
	If             string            `yaml:"if"`
	Needs          yaml.Node         `yaml:"needs"`
	TimeoutMinutes int               `yaml:"timeout-minutes"`
	Env            map[string]string `yaml:"env"`
	Steps          []prLaneStep      `yaml:"steps"`
}

func readPRLaneJobs(t *testing.T) map[string]prLaneJob {
	t.Helper()
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]prLaneJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow.Jobs
}

func prLaneNeeds(t *testing.T, job prLaneJob) []string {
	t.Helper()
	var needs []string
	switch job.Needs.Kind {
	case yaml.ScalarNode:
		needs = []string{job.Needs.Value}
	case yaml.SequenceNode:
		if err := job.Needs.Decode(&needs); err != nil {
			t.Fatal(err)
		}
	}
	return needs
}

// The six long-pole packages in the PR lane plan from the same shared
// duration history the full lane uses: Apex, VM, CLI, sema, LWC compile and
// Visualforce.
var prLaneHistory = []struct {
	job, artifact, env, dir, ifRange string
	base, count                      int
}{
	{"history-input", "apextest-history-input", "CI_APEXTEST_HISTORY_PATH", "/tmp/glade-apextest-duration-history", "", 0, 12},
	{"history-input", "vm-history-input", "CI_VM_HISTORY_PATH", "/tmp/glade-vm-duration-history", "", 12, 4},
	{"history-input", "gladecli-history-input", "CI_GLADECLI_HISTORY_PATH", "/tmp/glade-gladecli-duration-history", "", 16, 4},
	{"history-input", "sema-history-input", "CI_SEMA_HISTORY_PATH", "/tmp/glade-sema-duration-history", "", 20, 2},
	{"history-input", "lwc-compile-history-input", "CI_LWC_COMPILE_HISTORY_PATH", "/tmp/glade-lwc-compile-duration-history", "", 24, 12},
	{"history-input", "visualforce-history-input", "CI_VISUALFORCE_HISTORY_PATH", "/tmp/glade-visualforce-duration-history", "", 36, 8},
}

func TestCIPRLaneHistoryInputsRunOnPullRequests(t *testing.T) {
	jobs := readPRLaneJobs(t)
	for _, h := range prLaneHistory {
		job, ok := jobs[h.job]
		if !ok {
			t.Fatalf("job %s missing", h.job)
		}
		if strings.Contains(job.If, "github.event_name != 'pull_request'") {
			t.Errorf("%s skips pull requests: %s", h.job, job.If)
		}
		if !strings.Contains(job.If, "inputs.benchmark_cache_pair") {
			t.Errorf("%s lost its benchmark pair guard: %s", h.job, job.If)
		}
	}
}

func TestCIPRLaneConsumesOneSharedHistoryArtifact(t *testing.T) {
	jobs := readPRLaneJobs(t)
	pr, ok := jobs["pr-tests"]
	if !ok {
		t.Fatal("pr-tests missing")
	}
	needs := prLaneNeeds(t, pr)
	for _, h := range prLaneHistory {
		found := false
		for _, need := range needs {
			found = found || need == h.job
		}
		if !found {
			t.Errorf("pr-tests does not need %s (needs %v)", h.job, needs)
		}
		want := h.dir + "/" + strings.TrimSuffix(strings.TrimPrefix(h.dir, "/tmp/glade-"), "-duration-history") + "-duration-history.json"
		if got := pr.Env[h.env]; got != want {
			t.Errorf("pr-tests env %s = %q, want %q", h.env, got, want)
		}
		downloads := 0
		for _, step := range pr.Steps {
			if !strings.HasPrefix(step.Uses, "actions/download-artifact@") || step.With["name"] != h.artifact {
				continue
			}
			downloads++
			if step.With["path"] != h.dir {
				t.Errorf("%s downloads to %q, want %q", h.artifact, step.With["path"], h.dir)
			}
			if step.With["run-id"] != "" || step.With["github-token"] != "" {
				t.Errorf("%s must come from this run, not another run", h.artifact)
			}
			want := fmt.Sprintf("matrix.shard >= %d && matrix.shard < %d", h.base, h.base+h.count)
			if h.base == 0 {
				want = fmt.Sprintf("matrix.shard < %d", h.count)
			}
			if !strings.Contains(step.If, want) {
				t.Errorf("%s download if = %q, want it to contain %q", h.artifact, step.If, want)
			}
		}
		if downloads != 1 {
			t.Errorf("pr-tests downloads %s %d times, want once", h.artifact, downloads)
		}
	}
	// Shards that disagree on the plan would skip or repeat tests, and the PR
	// lane has no union check, so no shard may restore a history cache itself.
	for _, step := range pr.Steps {
		if !strings.HasPrefix(step.Uses, "actions/cache") {
			continue
		}
		for _, field := range []string{"key", "restore-keys", "path"} {
			if strings.Contains(step.With[field], "duration-history") {
				t.Errorf("pr-tests step %q restores duration history per shard (%s)", step.Name, field)
			}
		}
	}
	// Download ranges match the shard routes the job actually runs.
	spec := regexp.MustCompile(`(apex|vm|gladecli|sema|lwc-compile|visualforce):[a-z/]+:(\d+):(\d+)`)
	var run string
	for _, step := range pr.Steps {
		if strings.Contains(step.Run, "CI_APEXTEST_SHARDS") {
			run = step.Run
		}
	}
	routes := map[string][2]int{}
	for _, m := range spec.FindAllStringSubmatch(run, -1) {
		base, _ := strconv.Atoi(m[2])
		count, _ := strconv.Atoi(m[3])
		routes[m[1]] = [2]int{base, count}
	}
	for route, h := range map[string]int{"apex": 0, "vm": 1, "gladecli": 2, "sema": 3, "lwc-compile": 4, "visualforce": 5} {
		got, ok := routes[route]
		if !ok || got != [2]int{prLaneHistory[h].base, prLaneHistory[h].count} {
			t.Errorf("shard route %s = %v, history download covers %d+%d", route, got, prLaneHistory[h].base, prLaneHistory[h].count)
		}
	}
}

// The sema duration history has no dedicated full-lane input job: the sema
// shards restore their cache directly. The PR lane needs the history-input job
// to stage the same frozen artifact for its sema workers, or they plan from a
// deterministic fallback while the full lane plans from history.
func TestCIHistoryInputStagesSemaHistory(t *testing.T) {
	jobs := readPRLaneJobs(t)
	input, ok := jobs["history-input"]
	if !ok {
		t.Fatal("history-input job missing")
	}
	if got, want := input.Env["CI_SEMA_HISTORY_PATH"], "/tmp/glade-sema-duration-history/sema-duration-history.json"; got != want {
		t.Errorf("history-input env CI_SEMA_HISTORY_PATH = %q, want %q", got, want)
	}
	restores := 0
	for _, step := range input.Steps {
		if step.With["path"] == "/tmp/glade-sema-duration-history" && strings.HasPrefix(step.Uses, "actions/cache/restore@") {
			restores++
			if !strings.Contains(step.With["key"], "sema-duration-history-v1-") || !strings.Contains(step.With["key"], "env.CI_DURATION_HISTORY_CACHE_REF") {
				t.Errorf("sema history restore key = %q, want the duration-history cache key", step.With["key"])
			}
		}
	}
	if restores != 1 {
		t.Errorf("history-input restores the sema duration history %d times, want once", restores)
	}
	uploads := 0
	for _, step := range input.Steps {
		if !strings.HasPrefix(step.Uses, "actions/upload-artifact@") || step.With["name"] != "sema-history-input" {
			continue
		}
		uploads++
		if step.With["path"] != "ci-artifacts/sema-history-input/" {
			t.Errorf("sema-history-input upload path = %q, want the staged directory", step.With["path"])
		}
		if step.With["if-no-files-found"] != "error" {
			t.Errorf("sema-history-input upload must fail when nothing staged, got if-no-files-found %q", step.With["if-no-files-found"])
		}
	}
	if uploads != 1 {
		t.Errorf("history-input uploads sema-history-input %d times, want once", uploads)
	}
}

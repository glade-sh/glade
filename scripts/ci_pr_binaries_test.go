package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The four packages whose test binaries the full lane builds once and shares.
// The PR lane reuses those producers; vm and gladecli have none.
var prLaneBinaries = []struct {
	name, producer, pkg string
	base, count         int
}{
	{"apextest", "test-binaries-apextest", "github.com/glade-sh/glade/internal/apextest", 0, 12},
	{"sema", "test-binaries", "github.com/glade-sh/glade/internal/sema", 20, 2},
	{"lwc-compile", "test-binaries", "github.com/glade-sh/glade/internal/lwc/compile", 24, 12},
	{"visualforce", "test-binaries", "github.com/glade-sh/glade/internal/visualforce", 36, 8},
}

func runPRBinaries(t *testing.T, event, headRef, selected string) map[string]string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "selected.json")
	if err := os.WriteFile(file, []byte(selected), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "ci-pr-binaries.sh")
	cmd.Env = append(os.Environ(), "EVENT_NAME="+event, "HEAD_REF="+headRef, "CI_PR_SELECTED_FILE="+file)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-pr-binaries.sh: %v\n%s", err, out)
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("unexpected output line %q", line)
		}
		got[key] = value
	}
	return got
}

func TestCIPRBinarySelection(t *testing.T) {
	all := `["github.com/glade-sh/glade/internal/apextest","github.com/glade-sh/glade/internal/sema","github.com/glade-sh/glade/internal/lwc/compile","github.com/glade-sh/glade/internal/visualforce","github.com/glade-sh/glade/internal/vm"]`
	cases := []struct {
		name, event, head, selected string
		want                        map[string]string
	}{
		{"push builds all", "push", "", `[]`, map[string]string{"apextest": "true", "sema": "true", "lwc-compile": "true", "visualforce": "true"}},
		{"dispatch builds all", "workflow_dispatch", "", `[]`, map[string]string{"apextest": "true", "sema": "true", "lwc-compile": "true", "visualforce": "true"}},
		{"train builds all", "pull_request", "train/example", `[]`, map[string]string{"apextest": "true", "sema": "true", "lwc-compile": "true", "visualforce": "true"}},
		{"release builds all", "pull_request", "release/example", `[]`, map[string]string{"apextest": "true", "sema": "true", "lwc-compile": "true", "visualforce": "true"}},
		{"docs only builds none", "pull_request", "feature/x", `[]`, map[string]string{"apextest": "false", "sema": "false", "lwc-compile": "false", "visualforce": "false"}},
		{"one package", "pull_request", "feature/x", `["github.com/glade-sh/glade/internal/sema","github.com/glade-sh/glade/internal/vm"]`, map[string]string{"apextest": "false", "sema": "true", "lwc-compile": "false", "visualforce": "false"}},
		{"nested package", "pull_request", "feature/x", `["github.com/glade-sh/glade/internal/lwc/compile"]`, map[string]string{"apextest": "false", "sema": "false", "lwc-compile": "true", "visualforce": "false"}},
		{"everything selected", "pull_request", "feature/x", all, map[string]string{"apextest": "true", "sema": "true", "lwc-compile": "true", "visualforce": "true"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runPRBinaries(t, tc.event, tc.head, tc.selected)
			for key, want := range tc.want {
				if got[key] != want {
					t.Errorf("%s = %q, want %q (all: %v)", key, got[key], want, got)
				}
			}
			if len(got) != len(tc.want) {
				t.Errorf("outputs = %v, want exactly %v", got, tc.want)
			}
		})
	}
}

func TestCIPRLaneUsesSharedTestBinaries(t *testing.T) {
	jobs := readPRLaneJobs(t)
	pr := jobs["pr-tests"]
	needs := prLaneNeeds(t, pr)
	for _, producerName := range []string{"test-binaries-apextest", "test-binaries"} {
		producer, ok := jobs[producerName]
		if !ok {
			t.Fatalf("job %s missing", producerName)
		}
		if strings.Contains(producer.If, "github.event_name != 'pull_request'") {
			t.Errorf("%s skips pull requests: %s", producerName, producer.If)
		}
		found := false
		for _, need := range needs {
			found = found || need == producerName
		}
		if !found {
			t.Errorf("pr-tests does not need %s (needs %v)", producerName, needs)
		}
		selectSteps := 0
		for _, step := range producer.Steps {
			if step.Run != "" && strings.Contains(step.Run, "scripts/ci-pr-binaries.sh") {
				selectSteps++
			}
			if strings.HasPrefix(step.Uses, "actions/cache/save@") && !strings.Contains(step.If, "github.event_name != 'pull_request'") {
				t.Errorf("%s saves a cache on pull requests: %q", producerName, step.Name)
			}
		}
		if selectSteps != 1 {
			t.Errorf("%s selects binaries %d times, want once", producerName, selectSteps)
		}
	}
	for _, b := range prLaneBinaries {
		downloads := 0
		for _, step := range pr.Steps {
			name := step.With["name"]
			if !strings.HasPrefix(step.Uses, "actions/download-artifact@") || !strings.HasPrefix(name, "test-binaries-"+b.name+"-") {
				continue
			}
			downloads++
			wantName := "test-binaries-" + b.name + "-${{ github.run_id }}-${{ needs." + b.producer + ".outputs.artifact-attempt }}"
			if name != wantName {
				t.Errorf("%s artifact name = %q, want %q", b.name, name, wantName)
			}
			if step.With["path"] != "/tmp/glade-test-binaries" {
				t.Errorf("%s binary path = %q", b.name, step.With["path"])
			}
			if !strings.Contains(step.If, "steps.select.outputs.binary == '"+b.name+"'") {
				t.Errorf("%s download if = %q", b.name, step.If)
			}
		}
		if downloads != 1 {
			t.Errorf("pr-tests downloads the %s binary %d times, want once", b.name, downloads)
		}
	}
	// CI_TEST_BIN_DIR switches every route in the shard to a prebuilt binary,
	// so only a shard that downloaded one may set it.
	if _, ok := pr.Env["CI_TEST_BIN_DIR"]; ok {
		t.Error("pr-tests must not set CI_TEST_BIN_DIR for every shard")
	}
	exports := 0
	for _, step := range pr.Steps {
		if strings.Contains(step.Run, "CI_TEST_BIN_DIR=") {
			exports++
			if step.If != "${{ steps.select.outputs.binary != '' }}" {
				t.Errorf("CI_TEST_BIN_DIR export if = %q", step.If)
			}
		}
	}
	if exports != 1 {
		t.Errorf("CI_TEST_BIN_DIR is exported %d times, want once", exports)
	}
	// The select job computes the plan once; workers consume it.
	selectJob := jobs["pr-select"]
	found := false
	for _, step := range selectJob.Steps {
		if strings.Contains(step.Run, "scripts/ci-pr-select.sh") && strings.Contains(step.Run, "scripts/ci-pr-plan.py") {
			found = true
		}
	}
	if !found {
		t.Error("pr-select must select packages and emit the worker plan")
	}
	for _, step := range pr.Steps {
		if strings.Contains(step.Run, "scripts/ci-pr-select.sh") || strings.Contains(step.Run, "scripts/ci-pr-assign.py") {
			t.Error("worker recomputes selection or assignment")
		}
	}

}

// Exercise the workflow's actual selection step, including shard boundaries and
// a worker that has unrelated pool work but must not download a binary.
func TestCIPRLaneWorkerBinarySelection(t *testing.T) {
	pr := readPRLaneJobs(t)["pr-tests"]
	var script string
	for _, step := range pr.Steps {
		if step.Name == "Load planned packages" {
			script = step.Run
		}
	}
	if script == "" {
		t.Fatal("worker selection step missing")
	}
	for _, binary := range prLaneBinaries {
		for _, shard := range []int{binary.base - 1, binary.base, binary.base + binary.count - 1, binary.base + binary.count} {
			for _, present := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/shard-%d/present-%t", binary.name, shard, present), func(t *testing.T) {
					dir := t.TempDir()
					output := filepath.Join(dir, "output")
					packages := `["github.com/glade-sh/glade/internal/unrelated"]`
					if present {
						packages = fmt.Sprintf(`["%s"]`, binary.pkg)
					}
					cmd := exec.Command("bash", "-e", "-c", script)
					cmd.Env = append(os.Environ(), fmt.Sprintf("SHARD=%d", shard), "PLANNED_PACKAGES="+packages, "PLANNED_POOL=[]", "RUNNER_TEMP="+dir, "GITHUB_OUTPUT="+output)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("selection: %v\n%s", err, out)
					}
					got, err := os.ReadFile(output)
					if err != nil {
						t.Fatal(err)
					}
					want := ""
					if present && shard >= binary.base && shard < binary.base+binary.count {
						want = binary.name
					}
					if string(got) != "binary="+want+"\n" {
						t.Errorf("output = %q, want binary=%s", got, want)
					}
				})
			}
		}
	}
}

func TestCIPRLaneGoCachesAreReadOnly(t *testing.T) {
	pr := readPRLaneJobs(t)["pr-tests"]
	restores := map[string]int{}
	for _, step := range pr.Steps {
		if strings.HasPrefix(step.Uses, "actions/cache/save@") || strings.HasPrefix(step.Uses, "actions/cache@") {
			t.Errorf("pr-tests step %q writes a cache", step.Name)
		}
		if strings.HasPrefix(step.Uses, "actions/cache/restore@") {
			path := step.With["path"]
			restores[path]++
			if !strings.Contains(step.With["restore-keys"], "-ci-test-${{ hashFiles('go.sum') }}-") {
				t.Errorf("restore %q does not fall back to the shared ci-test cache", step.Name)
			}
			if strings.Contains(step.With["key"], "duration-history") {
				t.Errorf("restore %q restores duration history", step.Name)
			}
		}
	}
	if restores["~/go/pkg/mod"] != 1 || restores["~/.cache/go-build"] != 1 {
		t.Errorf("pr-tests cache restores = %v, want one module and one build cache", restores)
	}
}

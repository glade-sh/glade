package scripts

import (
	"slices"
	"strings"
	"testing"
)

// The ordinary-PR repository guard batch (CLI build, repoguard, scripts tests
// and the vendored-parser check) runs on its own worker instead of after PR
// worker 22's selection, and Required CI expects that worker's result.
func TestCIPRGuardWorkerRunsRepositoryGuards(t *testing.T) {
	jobs := readPRLaneJobs(t)
	guards, ok := jobs["pr-guards"]
	if !ok {
		t.Fatal("pr-guards job missing")
	}
	pr := jobs["pr-tests"]
	wantGuard := "${{ github.event_name == 'pull_request' && !startsWith(github.head_ref, 'train/') && !startsWith(github.head_ref, 'release/') }}"
	if guards.If != wantGuard {
		t.Fatalf("pr-guards if = %q, want the independent ordinary PR condition %q", guards.If, wantGuard)
	}
	if needs := prLaneNeeds(t, guards); len(needs) != 0 {
		t.Errorf("pr-guards needs = %v, want none so the guards start immediately", needs)
	}
	if guards.TimeoutMinutes != 45 {
		t.Errorf("pr-guards timeout-minutes = %d, want 45", guards.TimeoutMinutes)
	}
	for name, want := range map[string]string{"CGO_ENABLED": "1", "GOMAXPROCS": "2", "NO_COLOR": "1"} {
		if got := guards.Env[name]; got != want {
			t.Errorf("pr-guards env %s = %q, want %q", name, got, want)
		}
	}
	checkouts := 0
	actionlint := 0
	guardRuns := 0
	for _, step := range guards.Steps {
		switch {
		case strings.HasPrefix(step.Uses, "actions/checkout@"):
			checkouts++
			if step.With["fetch-depth"] != "0" || step.With["persist-credentials"] != "false" {
				t.Errorf("pr-guards checkout fetch-depth/persist-credentials = %q/%q, want 0/false for the merge-diff parser check", step.With["fetch-depth"], step.With["persist-credentials"])
			}
		case strings.HasPrefix(step.Uses, "actions/cache/save@") || strings.HasPrefix(step.Uses, "actions/cache@"):
			t.Errorf("pr-guards step %q writes a cache; the PR lane restores only", step.Name)
		case step.Run != "" && strings.Contains(step.Run, "install-actionlint.sh"):
			actionlint++
		case step.Run != "" && strings.Contains(step.Run, "./internal/repoguard ./scripts"):
			guardRuns++
			if step.If != "" {
				t.Errorf("pr-guards guard step is conditional (%q); the guards are mandatory", step.If)
			}
			for _, marker := range []string{
				`go build -o "$RUNNER_TEMP/glade" ./cmd/glade`,
				`go test -count=1 -timeout=0 ./internal/repoguard ./scripts`,
				`git diff --no-renames --name-only HEAD^1 HEAD | grep -q "^third_party/glade-apex-parser/"`,
				`(cd third_party/glade-apex-parser && go test -count=1 ./...)`,
			} {
				if !strings.Contains(step.Run, marker) {
					t.Errorf("pr-guards guard step lost %q:\n%s", marker, step.Run)
				}
			}
		}
	}
	if checkouts != 1 {
		t.Errorf("pr-guards checkouts = %d, want 1", checkouts)
	}
	if actionlint != 1 {
		t.Errorf("pr-guards installs actionlint %d times, want once for the scripts tests", actionlint)
	}
	if guardRuns != 1 {
		t.Fatalf("pr-guards guard batch found %d times, want once", guardRuns)
	}
	// Worker 22 keeps only its CLI authority and its package assignment.
	for _, step := range pr.Steps {
		if strings.Contains(step.Run, "./internal/repoguard") || strings.Contains(step.Run, `go build -o "$RUNNER_TEMP/glade"`) {
			t.Errorf("pr-tests step %q still runs the guard batch; it moved to pr-guards", step.Name)
		}
		if strings.Contains(step.Run, "install-actionlint.sh") {
			t.Errorf("pr-tests step %q still installs actionlint; the scripts tests moved to pr-guards", step.Name)
		}
	}
	// The aggregate must expect the split worker's result.
	required := jobs["required-ci"]
	if needs := prLaneNeeds(t, required); !slices.Contains(needs, "pr-guards") {
		t.Errorf("required-ci needs = %v, want pr-guards", needs)
	}
}

package scripts

import (
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const prAssignFixtureManifest = `{
  "version": 1,
  "lanes": {},
  "prDedicatedPackages": [
    "github.com/glade-sh/glade/internal/apextest",
    "github.com/glade-sh/glade/internal/gladecli",
    "github.com/glade-sh/glade/internal/lwc/compile",
    "github.com/glade-sh/glade/internal/playground",
    "github.com/glade-sh/glade/internal/repoguard",
    "github.com/glade-sh/glade/internal/sema",
    "github.com/glade-sh/glade/internal/server",
    "github.com/glade-sh/glade/internal/vm",
    "github.com/glade-sh/glade/internal/visualforce",
    "github.com/glade-sh/glade/scripts"
  ],
  "prPoolDedicatedSeconds": [
    {"package": "github.com/glade-sh/glade/internal/vm", "worker": 0, "seconds": 717},
    {"package": "github.com/glade-sh/glade/internal/vm", "worker": 1, "seconds": 550},
    {"package": "github.com/glade-sh/glade/internal/vm", "worker": 2, "seconds": 652},
    {"package": "github.com/glade-sh/glade/internal/vm", "worker": 3, "seconds": 846},
    {"package": "github.com/glade-sh/glade/internal/gladecli", "worker": 4, "seconds": 310},
    {"package": "github.com/glade-sh/glade/internal/gladecli", "worker": 5, "seconds": 236},
    {"package": "github.com/glade-sh/glade/internal/gladecli", "worker": 6, "seconds": 236},
    {"package": "github.com/glade-sh/glade/internal/gladecli", "worker": 7, "seconds": 314},
    {"package": "github.com/glade-sh/glade/internal/sema", "worker": 8, "seconds": 231},
    {"package": "github.com/glade-sh/glade/internal/sema", "worker": 9, "seconds": 647},
    {"package": "github.com/glade-sh/glade/internal/gladecli", "worker": 10, "seconds": 487}
  ],
  "prWorkerSeconds": {
    "github.com/glade-sh/glade/internal/storage": 370,
    "github.com/glade-sh/glade/internal/lwcbrowser": 124,
    "github.com/glade-sh/glade/internal/gladehome": 70,
    "github.com/glade-sh/glade/internal/dml": 41
  },
  "prWorkerSecondsDefault": 30
}
`

func runPRAssign(t *testing.T, manifest, selected string, worker, workers int) (string, int) {
	t.Helper()
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "ci-pr-assign.py",
		"--manifest", manifestPath,
		"--worker", strconv.Itoa(worker),
		"--workers", strconv.Itoa(workers))
	cmd.Stdin = strings.NewReader(selected)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run ci-pr-assign.py: %v\n%s", err, stderr.String())
		}
		code = exit.ExitCode()
	}
	if code != 0 && stdout.Len() != 0 {
		t.Fatalf("failed assignment still printed packages (%d):\n%s", code, stdout.String())
	}
	return stdout.String(), code
}

func prAssignSelection(packages ...string) string {
	encoded, err := json.Marshal(packages)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func prAssignPackages(output string) []string {
	var packages []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			packages = append(packages, line)
		}
	}
	slices.Sort(packages)
	return packages
}

// Every worker derives the same deterministic longest-first assignment from
// the same selection, the pool is covered exactly once, and the measured heavy
// packages never share a worker.
func TestCIPRAssignPartitionsPoolDeterministically(t *testing.T) {
	heavy := []string{
		"github.com/glade-sh/glade/internal/storage",
		"github.com/glade-sh/glade/internal/lwcbrowser",
		"github.com/glade-sh/glade/internal/gladehome",
		"github.com/glade-sh/glade/internal/dml",
	}
	selected := slices.Clone(heavy)
	selected = append(selected,
		"github.com/glade-sh/glade/internal/apextest",
		"github.com/glade-sh/glade/internal/vm",
		"github.com/glade-sh/glade/internal/gladecli",
		"github.com/glade-sh/glade/internal/sema",
		"github.com/glade-sh/glade/internal/lwc/compile",
		"github.com/glade-sh/glade/internal/visualforce",
		"github.com/glade-sh/glade/internal/server",
		"github.com/glade-sh/glade/internal/playground",
		"github.com/glade-sh/glade/internal/repoguard",
		"github.com/glade-sh/glade/scripts",
		"github.com/glade-sh/glade/internal/aura",
		"github.com/glade-sh/glade/internal/cliui",
		"github.com/glade-sh/glade/internal/config",
		"github.com/glade-sh/glade/internal/dap",
		"github.com/glade-sh/glade/internal/ir",
		"github.com/glade-sh/glade/internal/lsp",
		"github.com/glade-sh/glade/internal/tui",
	)
	slices.Sort(selected)
	var workers [][]string
	seen := map[string]int{}
	for worker := range 12 {
		out, code := runPRAssign(t, prAssignFixtureManifest, prAssignSelection(selected...), worker, 12)
		if code != 0 {
			t.Fatalf("worker %d assign failed (%d): %s", worker, code, out)
		}
		packages := prAssignPackages(out)
		workers = append(workers, packages)
		for _, pkg := range packages {
			seen[pkg]++
		}
	}
	want := slices.Clone(selected)
	for _, pkg := range []string{
		"github.com/glade-sh/glade/internal/apextest",
		"github.com/glade-sh/glade/internal/vm",
		"github.com/glade-sh/glade/internal/gladecli",
		"github.com/glade-sh/glade/internal/sema",
		"github.com/glade-sh/glade/internal/lwc/compile",
		"github.com/glade-sh/glade/internal/visualforce",
		"github.com/glade-sh/glade/internal/server",
		"github.com/glade-sh/glade/internal/playground",
		"github.com/glade-sh/glade/internal/repoguard",
		"github.com/glade-sh/glade/scripts",
	} {
		if seen[pkg] != 0 {
			t.Errorf("dedicated package %s was assigned to a pool worker", pkg)
		}
		want = slices.DeleteFunc(want, func(p string) bool { return p == pkg })
	}
	for pkg, count := range seen {
		if count != 1 {
			t.Errorf("package %s assigned %d times, want once", pkg, count)
		}
		want = slices.DeleteFunc(want, func(p string) bool { return p == pkg })
	}
	if len(want) != 0 {
		t.Errorf("pool packages missing from the assignment: %v", want)
	}
	holders := map[string]int{}
	for worker, packages := range workers {
		for _, pkg := range packages {
			if slices.Contains(heavy, pkg) {
				if other, ok := holders[pkg]; ok {
					t.Fatalf("%s listed on workers %d and %d", pkg, other, worker)
				}
				holders[pkg] = worker
			}
		}
	}
	if len(holders) != len(heavy) {
		t.Fatalf("heavy packages assigned = %v, want all of %v", holders, heavy)
	}
	distinct := map[int]bool{}
	for _, worker := range holders {
		if distinct[worker] {
			t.Fatalf("two heavy measured packages share worker %d", worker)
		}
		distinct[worker] = true
	}
	// The same inputs must produce the same assignment on a second run.
	for worker := range 12 {
		first, _ := runPRAssign(t, prAssignFixtureManifest, prAssignSelection(selected...), worker, 12)
		second, _ := runPRAssign(t, prAssignFixtureManifest, prAssignSelection(selected...), worker, 12)
		if first != second {
			t.Fatalf("worker %d assignment is not deterministic:\n%s\n%s", worker, first, second)
		}
	}
}

// Longest-first placement with the least-loaded worker and lowest-index
// tie-break is the whole balancing contract for a small selection.
func TestCIPRAssignPlacesLongestFirst(t *testing.T) {
	selected := prAssignSelection(
		"github.com/glade-sh/glade/internal/storage",
		"github.com/glade-sh/glade/internal/lwcbrowser",
		"github.com/glade-sh/glade/internal/gladehome",
	)
	for worker, want := range map[int][]string{
		0: {"github.com/glade-sh/glade/internal/storage"},
		1: {"github.com/glade-sh/glade/internal/lwcbrowser"},
		2: {"github.com/glade-sh/glade/internal/gladehome"},
		3: {},
	} {
		out, code := runPRAssign(t, prAssignFixtureManifest, selected, worker, 12)
		if code != 0 {
			t.Fatalf("worker %d assign failed (%d): %s", worker, code, out)
		}
		if got := prAssignPackages(out); !slices.Equal(got, want) {
			t.Errorf("worker %d = %v, want %v", worker, got, want)
		}
	}
}

// The pool balancer must account for the dedicated-route work a pool worker
// already runs when its route's package is selected. In the PR 253 run the
// unseeded balancer put storage, the heaviest pool package, on the worker that
// also carries VM shard 0, and that stack became the 24m18s tail. With the
// measured VM, CLI and sema shard seeds, storage moves to the only pool worker
// without a dedicated route, and the workers carrying the heaviest dedicated
// shards keep no pool packages at all.
func TestCIPRAssignBalancesPoolAgainstDedicatedLoad(t *testing.T) {
	selected := prAssignSelection(
		"github.com/glade-sh/glade/internal/vm",
		"github.com/glade-sh/glade/internal/gladecli",
		"github.com/glade-sh/glade/internal/sema",
		"github.com/glade-sh/glade/internal/storage",
		"github.com/glade-sh/glade/internal/lwcbrowser",
		"github.com/glade-sh/glade/internal/gladehome",
		"github.com/glade-sh/glade/internal/dml",
	)
	assigned := make(map[int][]string, 12)
	for worker := range 12 {
		out, code := runPRAssign(t, prAssignFixtureManifest, selected, worker, 12)
		if code != 0 {
			t.Fatalf("worker %d assign failed (%d): %s", worker, code, out)
		}
		assigned[worker] = prAssignPackages(out)
	}
	storage := "github.com/glade-sh/glade/internal/storage"
	storageWorker := -1
	for worker, packages := range assigned {
		if slices.Contains(packages, storage) {
			storageWorker = worker
		}
	}
	if storageWorker != 11 {
		t.Fatalf("storage landed on pool worker %d, want 11 (the only worker without a dedicated route)", storageWorker)
	}
	// Worker 0 carries VM shard 0 (717 s, the measured tail) and worker 3 the
	// heaviest dedicated shard (846 s); neither may also carry pool work here.
	for _, worker := range []int{0, 3} {
		if packages := assigned[worker]; len(packages) != 0 {
			t.Errorf("pool worker %d kept pool packages %v on top of its dedicated shard", worker, packages)
		}
	}
}

// Dedicated seeds break ties by the lowest worker index, seeds steer pool work
// away from the heavier-seeded worker, and a worker whose route package the
// selection does not include (or that has no table entry at all) starts at the
// zero default.
func TestCIPRAssignDedicatedSeedsTieAndDefault(t *testing.T) {
	tieManifest := `{
  "version": 1,
  "lanes": {},
  "prDedicatedPackages": [
    "github.com/glade-sh/glade/internal/vm",
    "github.com/glade-sh/glade/internal/sema"
  ],
  "prPoolDedicatedSeconds": [
    {"package": "github.com/glade-sh/glade/internal/vm", "worker": 0, "seconds": 100},
    {"package": "github.com/glade-sh/glade/internal/vm", "worker": 1, "seconds": 100},
    {"package": "github.com/glade-sh/glade/internal/vm", "worker": 2, "seconds": 400}
  ],
  "prWorkerSeconds": {},
  "prWorkerSecondsDefault": 30
}
`
	steerManifest := `{
  "version": 1,
  "lanes": {},
  "prDedicatedPackages": [
    "github.com/glade-sh/glade/internal/vm",
    "github.com/glade-sh/glade/internal/sema"
  ],
  "prPoolDedicatedSeconds": [
    {"package": "github.com/glade-sh/glade/internal/vm", "worker": 0, "seconds": 150},
    {"package": "github.com/glade-sh/glade/internal/sema", "worker": 2, "seconds": 400}
  ],
  "prWorkerSeconds": {},
  "prWorkerSecondsDefault": 30
}
`
	cases := []struct {
		name     string
		manifest string
		selected string
		want     map[int][]string
	}{
		{
			// Workers 0 and 1 carry the same VM seed and worker 2 a heavier
			// one, so the first equal package tie-breaks to worker 0 and the
			// second follows the other tied worker instead of stacking.
			name:     "equal seeds tie to the lowest worker",
			manifest: tieManifest,
			selected: prAssignSelection(
				"github.com/glade-sh/glade/internal/vm",
				"github.com/glade-sh/glade/internal/aura",
				"github.com/glade-sh/glade/internal/cliui",
			),
			want: map[int][]string{
				0: {"github.com/glade-sh/glade/internal/aura"},
				1: {"github.com/glade-sh/glade/internal/cliui"},
				2: {},
			},
		},
		{
			// Worker 0's VM seed keeps it out of the work while the workers
			// without an applicable seed split it; worker 2's sema entry
			// contributes nothing because sema is not selected.
			name:     "unequal seeds steer pool work",
			manifest: steerManifest,
			selected: prAssignSelection(
				"github.com/glade-sh/glade/internal/vm",
				"github.com/glade-sh/glade/internal/aura",
				"github.com/glade-sh/glade/internal/cliui",
			),
			want: map[int][]string{
				0: {},
				1: {"github.com/glade-sh/glade/internal/aura"},
				2: {"github.com/glade-sh/glade/internal/cliui"},
			},
		},
		{
			// vm is not selected, so worker 0's route contributes no seed and
			// worker 1, which has no table entry, defaults to zero; both take
			// work before worker 2, whose selected sema route seeds it.
			name:     "unselected route and missing entry default to zero",
			manifest: steerManifest,
			selected: prAssignSelection(
				"github.com/glade-sh/glade/internal/sema",
				"github.com/glade-sh/glade/internal/aura",
				"github.com/glade-sh/glade/internal/cliui",
			),
			want: map[int][]string{
				0: {"github.com/glade-sh/glade/internal/aura"},
				1: {"github.com/glade-sh/glade/internal/cliui"},
				2: {},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for worker, packages := range tc.want {
				out, code := runPRAssign(t, tc.manifest, tc.selected, worker, 3)
				if code != 0 {
					t.Fatalf("worker %d assign failed (%d): %s", worker, code, out)
				}
				if got := prAssignPackages(out); !slices.Equal(got, packages) {
					t.Errorf("worker %d = %v, want %v", worker, got, packages)
				}
			}
		})
	}
}

// The shared fixture's dedicated table covers the 12-worker pool, so this
// variant keeps every field the assigner requires but seeds no dedicated work:
// a two-worker assignment then exercises only the default estimate.
const prAssignNoDedicatedManifest = `{
  "version": 1,
  "lanes": {},
  "prDedicatedPackages": [],
  "prPoolDedicatedSeconds": [],
  "prWorkerSeconds": {
    "github.com/glade-sh/glade/internal/storage": 370
  },
  "prWorkerSecondsDefault": 30
}
`

// Packages without a measured entry still run; the conservative default
// estimate keeps them off the measured heavy worker instead of dropping or
// stacking them.
func TestCIPRAssignUsesDefaultSecondsForUnmeasuredPackages(t *testing.T) {
	selected := prAssignSelection(
		"github.com/glade-sh/glade/internal/storage",
		"github.com/glade-sh/glade/internal/aura",
		"github.com/glade-sh/glade/internal/cliui",
	)
	want := map[int][]string{
		0: {"github.com/glade-sh/glade/internal/storage"},
		1: {"github.com/glade-sh/glade/internal/aura", "github.com/glade-sh/glade/internal/cliui"},
	}
	for worker, packages := range want {
		out, code := runPRAssign(t, prAssignNoDedicatedManifest, selected, worker, 2)
		if code != 0 {
			t.Fatalf("worker %d assign failed (%d): %s", worker, code, out)
		}
		if got := prAssignPackages(out); !slices.Equal(got, packages) {
			t.Errorf("worker %d = %v, want %v", worker, got, packages)
		}
	}
}

func TestCIPRAssignFailsClosed(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		selected string
		worker   int
		workers  int
	}{
		{"worker index out of range", prAssignFixtureManifest, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 12, 12},
		{"negative worker index", prAssignFixtureManifest, prAssignSelection("github.com/glade-sh/glade/internal/aura"), -1, 12},
		{"zero workers", prAssignFixtureManifest, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 0},
		{"duplicate selection entry", prAssignFixtureManifest, prAssignSelection("github.com/glade-sh/glade/internal/aura", "github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"malformed selection", prAssignFixtureManifest, "not json", 0, 12},
		{"missing dedicated list", `{"version":1,"lanes":{},"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"missing seconds table", `{"version":1,"lanes":{},"prDedicatedPackages":[],"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"missing default estimate", `{"version":1,"lanes":{},"prDedicatedPackages":[],"prWorkerSeconds":{}}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"nonpositive default estimate", `{"version":1,"lanes":{},"prDedicatedPackages":[],"prWorkerSeconds":{},"prWorkerSecondsDefault":0}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"missing dedicated seconds table", `{"version":1,"lanes":{},"prDedicatedPackages":[],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds not a list", `{"version":1,"lanes":{},"prDedicatedPackages":[],"prPoolDedicatedSeconds":{},"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds entry not an object", `{"version":1,"lanes":{},"prDedicatedPackages":[],"prPoolDedicatedSeconds":[7],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds entry missing seconds", `{"version":1,"lanes":{},"prDedicatedPackages":["github.com/glade-sh/glade/internal/vm"],"prPoolDedicatedSeconds":[{"package":"github.com/glade-sh/glade/internal/vm","worker":0}],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds entry extra key", `{"version":1,"lanes":{},"prDedicatedPackages":["github.com/glade-sh/glade/internal/vm"],"prPoolDedicatedSeconds":[{"package":"github.com/glade-sh/glade/internal/vm","worker":0,"seconds":10,"source":"run"}],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds package not dedicated", `{"version":1,"lanes":{},"prDedicatedPackages":[],"prPoolDedicatedSeconds":[{"package":"github.com/glade-sh/glade/internal/storage","worker":0,"seconds":10}],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds worker out of range", `{"version":1,"lanes":{},"prDedicatedPackages":["github.com/glade-sh/glade/internal/vm"],"prPoolDedicatedSeconds":[{"package":"github.com/glade-sh/glade/internal/vm","worker":12,"seconds":10}],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds negative worker", `{"version":1,"lanes":{},"prDedicatedPackages":["github.com/glade-sh/glade/internal/vm"],"prPoolDedicatedSeconds":[{"package":"github.com/glade-sh/glade/internal/vm","worker":-1,"seconds":10}],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds non-integer worker", `{"version":1,"lanes":{},"prDedicatedPackages":["github.com/glade-sh/glade/internal/vm"],"prPoolDedicatedSeconds":[{"package":"github.com/glade-sh/glade/internal/vm","worker":"0","seconds":10}],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds nonpositive estimate", `{"version":1,"lanes":{},"prDedicatedPackages":["github.com/glade-sh/glade/internal/vm"],"prPoolDedicatedSeconds":[{"package":"github.com/glade-sh/glade/internal/vm","worker":0,"seconds":0}],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
		{"dedicated seconds duplicate worker package", `{"version":1,"lanes":{},"prDedicatedPackages":["github.com/glade-sh/glade/internal/vm"],"prPoolDedicatedSeconds":[{"package":"github.com/glade-sh/glade/internal/vm","worker":0,"seconds":10},{"package":"github.com/glade-sh/glade/internal/vm","worker":0,"seconds":11}],"prWorkerSeconds":{},"prWorkerSecondsDefault":30}`, prAssignSelection("github.com/glade-sh/glade/internal/aura"), 0, 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, code := runPRAssign(t, tc.manifest, tc.selected, tc.worker, tc.workers)
			if code == 0 {
				t.Fatalf("invalid input accepted: %s", out)
			}
		})
	}
}

type prPoolDedicatedEntry struct {
	Package string `json:"package"`
	Worker  int    `json:"worker"`
	Seconds int    `json:"seconds"`
}

type prPoolDedicatedKey struct {
	worker int
	pkg    string
}

type prPackageManifest struct {
	Lanes                        map[string][]string    `json:"lanes"`
	PRDedicatedPackages          []string               `json:"prDedicatedPackages"`
	PRPoolDedicatedSeconds       []prPoolDedicatedEntry `json:"prPoolDedicatedSeconds"`
	PRPoolDedicatedSecondsSource string                 `json:"prPoolDedicatedSecondsSource"`
	PRWorkerSeconds              map[string]int         `json:"prWorkerSeconds"`
	PRWorkerSecondsDefault       int                    `json:"prWorkerSecondsDefault"`
	PRWorkerSecondsSource        string                 `json:"prWorkerSecondsSource"`
}

func readPRPackageManifest(t *testing.T) prPackageManifest {
	t.Helper()
	data, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest prPackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

// The pool excludes exactly the packages the workflow routes to dedicated
// workers: the shard-spec loop, worker 44's server/playground route and the
// guard worker's repoguard/scripts batch. A drift in either direction would
// run one package twice or not at all.
func TestCIPRDedicatedPackagesMatchWorkflowRoutes(t *testing.T) {
	manifest := readPRPackageManifest(t)
	if len(manifest.PRDedicatedPackages) == 0 {
		t.Fatal("ci-package-lanes.json has no prDedicatedPackages")
	}
	jobs := readPRLaneJobs(t)
	var run string
	for _, step := range jobs["pr-tests"].Steps {
		if strings.Contains(step.Run, "CI_APEXTEST_SHARDS") {
			run = step.Run
		}
	}
	if run == "" {
		t.Fatal("pr-tests package step missing")
	}
	dedicated := map[string]bool{}
	spec := regexp.MustCompile(`(?:apex|vm|gladecli|sema|lwc-compile|visualforce):([a-z/]+):(\d+):(\d+)`)
	for _, match := range spec.FindAllStringSubmatch(run, -1) {
		dedicated["github.com/glade-sh/glade/internal/"+match[1]] = true
	}
	for _, pkg := range []string{"github.com/glade-sh/glade/internal/server", "github.com/glade-sh/glade/internal/playground"} {
		if !strings.Contains(run, `. == "`+pkg+`"`) {
			t.Errorf("worker 44 route no longer owns %s", pkg)
		}
		dedicated[pkg] = true
	}
	var guards string
	for _, step := range jobs["pr-guards"].Steps {
		if strings.Contains(step.Run, "./internal/repoguard ./scripts") {
			guards = step.Run
		}
	}
	if guards == "" {
		t.Fatal("pr-guards batch step missing")
	}
	dedicated["github.com/glade-sh/glade/internal/repoguard"] = true
	dedicated["github.com/glade-sh/glade/scripts"] = true
	var want []string
	for pkg := range dedicated {
		want = append(want, pkg)
	}
	slices.Sort(want)
	got := slices.Clone(manifest.PRDedicatedPackages)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("prDedicatedPackages = %v, want the workflow's dedicated routes %v", got, want)
	}
	// Every dedicated entry is a real package in the manifest lanes, and the
	// pool workers derive their slice with the published worker count.
	owned := map[string]bool{}
	for _, packages := range manifest.Lanes {
		for _, pkg := range packages {
			owned[pkg] = true
		}
	}
	for _, pkg := range manifest.PRDedicatedPackages {
		if !owned[pkg] {
			t.Errorf("dedicated package %s is not in any lane", pkg)
		}
	}
	for _, marker := range []string{
		"scripts/ci-pr-assign.py",
		"SHARD >= 12 && SHARD < 24",
		`--worker "$((SHARD - 12))"`,
		"--workers 12",
	} {
		if !strings.Contains(run, marker) {
			t.Errorf("pool assignment missing %q:\n%s", marker, run)
		}
	}
}

// The balancing table records the measured train durations for the remaining
// packages: the storage, scripts and lwcbrowser values from the CI review are
// 370, 192 and 124 seconds, and scripts runs on the guard worker, so the pool
// table pins storage and lwcbrowser.
func TestCIPRWorkerSecondsInventory(t *testing.T) {
	manifest := readPRPackageManifest(t)
	if manifest.PRWorkerSecondsDefault <= 0 {
		t.Errorf("prWorkerSecondsDefault = %d, want a positive conservative estimate", manifest.PRWorkerSecondsDefault)
	}
	if strings.TrimSpace(manifest.PRWorkerSecondsSource) == "" {
		t.Error("prWorkerSecondsSource is empty; the table must state its measurement origin")
	}
	owned := map[string]bool{}
	for _, packages := range manifest.Lanes {
		for _, pkg := range packages {
			owned[pkg] = true
		}
	}
	dedicated := map[string]bool{}
	for _, pkg := range manifest.PRDedicatedPackages {
		dedicated[pkg] = true
	}
	for pkg, seconds := range manifest.PRWorkerSeconds {
		if !owned[pkg] {
			t.Errorf("seconds entry %s is not a package in the lanes", pkg)
		}
		if dedicated[pkg] {
			t.Errorf("seconds entry %s belongs to a dedicated worker, not the pool", pkg)
		}
		if seconds <= 0 {
			t.Errorf("seconds entry %s = %d, want positive", pkg, seconds)
		}
	}
	for pkg, want := range map[string]int{
		"github.com/glade-sh/glade/internal/storage":    370,
		"github.com/glade-sh/glade/internal/lwcbrowser": 124,
	} {
		if got := manifest.PRWorkerSeconds[pkg]; got != want {
			t.Errorf("measured seconds for %s = %d, want the train measurement %d", pkg, got, want)
		}
	}
}

// The dedicated-load table records the dedicated work each PR pool worker
// already runs when its route's package is selected: the PR 253 Required CI
// run 44461362525 (the run the CI review measured) recorded VM shard 0 at
// 716.968 s on worker 12 in its step log, and the same run's step logs measure
// the other VM, CLI and sema shards and worker 22's doctor authority. The test
// pins the rounded measurements so a rebalance is a deliberate manifest change
// with a new stated origin, not silent drift.
func TestCIPRPoolDedicatedSecondsInventory(t *testing.T) {
	manifest := readPRPackageManifest(t)
	if strings.TrimSpace(manifest.PRPoolDedicatedSecondsSource) == "" {
		t.Error("prPoolDedicatedSecondsSource is empty; the table must state its measurement origin")
	}
	owned := map[string]bool{}
	for _, packages := range manifest.Lanes {
		for _, pkg := range packages {
			owned[pkg] = true
		}
	}
	dedicated := map[string]bool{}
	for _, pkg := range manifest.PRDedicatedPackages {
		dedicated[pkg] = true
	}
	got := map[prPoolDedicatedKey]int{}
	for _, entry := range manifest.PRPoolDedicatedSeconds {
		key := prPoolDedicatedKey{worker: entry.Worker, pkg: entry.Package}
		if _, repeat := got[key]; repeat {
			t.Errorf("dedicated seconds repeat pool worker %d for %s", entry.Worker, entry.Package)
		}
		if !owned[entry.Package] {
			t.Errorf("dedicated seconds entry %s is not a package in the lanes", entry.Package)
		}
		if !dedicated[entry.Package] {
			t.Errorf("dedicated seconds entry %s is not a dedicated route package", entry.Package)
		}
		if entry.Worker < 0 || entry.Worker > 11 {
			t.Errorf("dedicated seconds entry %s names pool worker %d, outside 0..11", entry.Package, entry.Worker)
		}
		if entry.Seconds < 1 {
			t.Errorf("dedicated seconds entry %s on worker %d = %d, want positive", entry.Package, entry.Worker, entry.Seconds)
		}
		got[key] = entry.Seconds
	}
	want := map[prPoolDedicatedKey]int{}
	for pkg, workers := range map[string]map[int]int{
		"github.com/glade-sh/glade/internal/vm":       {0: 717, 1: 550, 2: 652, 3: 846},
		"github.com/glade-sh/glade/internal/gladecli": {4: 310, 5: 236, 6: 236, 7: 314, 10: 487},
		"github.com/glade-sh/glade/internal/sema":     {8: 231, 9: 647},
	} {
		for worker, seconds := range workers {
			want[prPoolDedicatedKey{worker: worker, pkg: pkg}] = seconds
		}
	}
	if !maps.Equal(got, want) {
		t.Fatalf("prPoolDedicatedSeconds = %v, want the run 44461362525 measurements %v", got, want)
	}
}

// The dedicated-load table must cover exactly the dedicated routes the
// workflow puts on the pool workers: the vm, gladecli and sema shard specs of
// the pr-tests route loop, each gated on its package being selected, plus
// worker 22's CLI doctor authority, which runs under the same gladecli
// selection guard. Worker 23 carries no dedicated route, so it has no entry and
// is the natural home for the heaviest pool package.
func TestCIPRPoolDedicatedSecondsMatchWorkflowRoutes(t *testing.T) {
	manifest := readPRPackageManifest(t)
	jobs := readPRLaneJobs(t)
	var run string
	for _, step := range jobs["pr-tests"].Steps {
		if strings.Contains(step.Run, "CI_APEXTEST_SHARDS") {
			run = step.Run
		}
	}
	if run == "" {
		t.Fatal("pr-tests package step missing")
	}
	want := map[prPoolDedicatedKey]bool{}
	spec := regexp.MustCompile(`(?:apex|vm|gladecli|sema|lwc-compile|visualforce):([a-z/]+):(\d+):(\d+)`)
	for _, match := range spec.FindAllStringSubmatch(run, -1) {
		pkg := match[1]
		base, baseErr := strconv.Atoi(match[2])
		count, countErr := strconv.Atoi(match[3])
		if baseErr != nil || countErr != nil {
			t.Fatalf("route spec %s:%s has a non-integer base or count", match[2], match[3])
		}
		// Only the routes whose workers fall inside the pool range 12..23
		// contribute dedicated load; the apex, lwc-compile and visualforce
		// specs run on workers outside the pool.
		if base < 12 || base+count > 24 {
			continue
		}
		for worker := base - 12; worker < base-12+count; worker++ {
			want[prPoolDedicatedKey{worker: worker, pkg: "github.com/glade-sh/glade/internal/" + pkg}] = true
		}
	}
	// Worker 22 runs the four CLI doctor tests when the selection includes
	// gladecli, so pool worker 10 carries a gladecli-keyed entry as well.
	if !strings.Contains(run, `[[ "$SHARD" == 22 ]]`) || !strings.Contains(run, `index("github.com/glade-sh/glade/internal/gladecli") != null`) {
		t.Fatal("worker 22 doctor authority route missing from the pr-tests step")
	}
	want[prPoolDedicatedKey{worker: 10, pkg: "github.com/glade-sh/glade/internal/gladecli"}] = true
	got := map[prPoolDedicatedKey]bool{}
	for _, entry := range manifest.PRPoolDedicatedSeconds {
		got[prPoolDedicatedKey{worker: entry.Worker, pkg: entry.Package}] = true
	}
	if !maps.Equal(got, want) {
		t.Fatalf("prPoolDedicatedSeconds routes = %v, want the workflow's pool-worker dedicated routes %v", got, want)
	}
}

// The pool workers of a real full selection stay inside the memory contract:
// each PR worker keeps GOMAXPROCS 2 and the two memory-heavy measured packages
// land on different workers.
func TestCIPRWorkerAssignmentsKeepWorkerLimits(t *testing.T) {
	manifest := readPRPackageManifest(t)
	owned := map[string]bool{}
	for _, packages := range manifest.Lanes {
		for _, pkg := range packages {
			owned[pkg] = true
		}
	}
	dedicated := map[string]bool{}
	for _, pkg := range manifest.PRDedicatedPackages {
		dedicated[pkg] = true
	}
	var selected []string
	for pkg := range owned {
		if !dedicated[pkg] {
			selected = append(selected, pkg)
		}
	}
	slices.Sort(selected)
	var workflow struct {
		Jobs map[string]struct {
			Env map[string]string `yaml:"env"`
		} `yaml:"jobs"`
	}
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	if got := workflow.Jobs["pr-tests"].Env["GOMAXPROCS"]; got != "2" {
		t.Errorf("pr-tests GOMAXPROCS = %q, want 2", got)
	}
	storageWorker, browserWorker := -1, -1
	for worker := range 12 {
		out, code := runPRAssign(t, prAssignFile(t), prAssignSelection(selected...), worker, 12)
		if code != 0 {
			t.Fatalf("worker %d assign failed (%d): %s", worker, code, out)
		}
		for _, pkg := range prAssignPackages(out) {
			switch pkg {
			case "github.com/glade-sh/glade/internal/storage":
				storageWorker = worker
			case "github.com/glade-sh/glade/internal/lwcbrowser":
				browserWorker = worker
			}
		}
	}
	if storageWorker < 0 || browserWorker < 0 || storageWorker == browserWorker {
		t.Fatalf("storage and lwcbrowser share worker %d/%d; the memory-heavy packages must stay separate", storageWorker, browserWorker)
	}
}

func prAssignFile(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

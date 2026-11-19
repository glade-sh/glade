package scripts

import (
	"encoding/json"
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
		out, code := runPRAssign(t, prAssignFixtureManifest, selected, worker, 2)
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

type prPackageManifest struct {
	Lanes                  map[string][]string `json:"lanes"`
	PRDedicatedPackages    []string            `json:"prDedicatedPackages"`
	PRWorkerSeconds        map[string]int      `json:"prWorkerSeconds"`
	PRWorkerSecondsDefault int                 `json:"prWorkerSecondsDefault"`
	PRWorkerSecondsSource  string              `json:"prWorkerSecondsSource"`
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
		if !strings.Contains(run, `select(. == "`+pkg+`"`) {
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

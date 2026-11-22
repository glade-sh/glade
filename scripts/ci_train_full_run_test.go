package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ciScenario is one way the CI workflow can start.
type ciScenario struct {
	name, event, headRef string
}

var ciScenarios = []ciScenario{
	{"push", "push", ""},
	{"pull_request", "pull_request", "feature/example"},
	{"train pull_request", "pull_request", "train/example"},
	{"release pull_request", "pull_request", "release/example"},
}

var (
	ciIfEvent      = regexp.MustCompile(`github\.event_name\s*(==|!=)\s*'(\w+)'`)
	ciIfHeadRef    = regexp.MustCompile(`startsWith\(github\.head_ref,\s*'([^']+)'\)`)
	ciIfInputs     = regexp.MustCompile(`inputs\.benchmark_cache_pair\s*(>=|<=|>|<|==)\s*\d+`)
	ciIfStatusCall = regexp.MustCompile(`\b(success|always)\(\)`)
)

// ciIfApplies evaluates a job-level `if` for a scenario that is not a manual
// dispatch. It accepts only the atoms the workflow uses, so an unknown form
// fails the test instead of being guessed at.
func ciIfApplies(t *testing.T, expr string, sc ciScenario) bool {
	t.Helper()
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true
	}
	expr = strings.TrimSuffix(strings.TrimPrefix(expr, "${{"), "}}")
	expr = ciIfEvent.ReplaceAllStringFunc(expr, func(m string) string {
		sub := ciIfEvent.FindStringSubmatch(m)
		if (sub[2] == sc.event) == (sub[1] == "==") {
			return "T"
		}
		return "F"
	})
	expr = ciIfHeadRef.ReplaceAllStringFunc(expr, func(m string) string {
		if strings.HasPrefix(sc.headRef, ciIfHeadRef.FindStringSubmatch(m)[1]) {
			return "T"
		}
		return "F"
	})
	expr = ciIfInputs.ReplaceAllString(expr, "T")
	// Exercise the maximum selected PR routes when checking the job budget.
	expr = strings.ReplaceAll(expr, "needs.pr-select.outputs.smoke == 'true'", "T")
	expr = strings.ReplaceAll(expr, "needs.pr-select.outputs.browser == 'true'", "T")
	// These scenarios exercise the normal full/fast paths: the receipt gate
	// succeeds without reuse, and each ordinary prerequisite succeeds.
	expr = strings.ReplaceAll(expr, "needs.tree-reuse-gate.result != 'success'", "F")
	expr = strings.ReplaceAll(expr, "needs.tree-reuse-gate.result == 'success'", "T")
	expr = strings.ReplaceAll(expr, "needs.tree-reuse-gate.outputs.reuse != 'true'", "T")
	expr = strings.ReplaceAll(expr, "needs.tree-reuse-gate.outputs.reuse == 'true'", "F")
	expr = regexp.MustCompile(`needs\.[a-zA-Z0-9_-]+\.result == 'success'`).ReplaceAllString(expr, "T")
	// A scenario is not a cancelled run.
	expr = strings.ReplaceAll(expr, "!cancelled()", "T")
	expr = ciIfStatusCall.ReplaceAllString(expr, "T")
	toks := regexp.MustCompile(`&&|\|\||[()!TF]|\S`).FindAllString(expr, -1)
	pos := 0
	var parseOr func() bool
	parseAtom := func() bool {
		if pos >= len(toks) {
			t.Fatalf("unreadable if %q", expr)
		}
		tok := toks[pos]
		pos++
		switch tok {
		case "T":
			return true
		case "F":
			return false
		case "!":
			// handled by caller through recursion below
		case "(":
			v := parseOr()
			if pos >= len(toks) || toks[pos] != ")" {
				t.Fatalf("unbalanced if %q", expr)
			}
			pos++
			return v
		}
		t.Fatalf("unsupported token %q in if %q", tok, expr)
		return false
	}
	var parseNot func() bool
	parseNot = func() bool {
		if pos < len(toks) && toks[pos] == "!" {
			pos++
			return !parseNot()
		}
		return parseAtom()
	}
	parseAnd := func() bool {
		v := parseNot()
		for pos < len(toks) && toks[pos] == "&&" {
			pos++
			r := parseNot()
			v = v && r
		}
		return v
	}
	parseOr = func() bool {
		v := parseAnd()
		for pos < len(toks) && toks[pos] == "||" {
			pos++
			r := parseAnd()
			v = v || r
		}
		return v
	}
	v := parseOr()
	if pos != len(toks) {
		t.Fatalf("trailing tokens in if %q", expr)
	}
	return v
}

func ciJobWidth(t *testing.T, name string, node yaml.Node) int {
	t.Helper()
	if node.Kind == 0 {
		return 1
	}
	if node.Kind == yaml.ScalarNode {
		if name != "pr-tests" || node.Value != "${{ fromJSON(needs.pr-select.outputs.matrix) }}" {
			t.Fatalf("job %s has an unrecognized matrix expression %q", name, node.Value)
		}
		data, err := os.ReadFile("ci-package-lanes.json")
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Dedicated []string       `json:"prDedicatedPackages"`
			Pool      map[string]any `json:"prWorkerSeconds"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		selected := make(map[string]bool)
		for _, pkg := range manifest.Dedicated {
			selected[pkg] = true
		}
		for pkg := range manifest.Pool {
			selected[pkg] = true
		}
		packages := make([]string, 0, len(selected))
		for pkg := range selected {
			packages = append(packages, pkg)
		}
		sort.Strings(packages)
		input, err := json.Marshal(packages)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("python3", "ci-pr-plan.py")
		cmd.Stdin = strings.NewReader(string(input))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("plan maximal PR selection: %v: %s", err, out)
		}
		var plan struct {
			Include []json.RawMessage `json:"include"`
		}
		if err := json.Unmarshal(out, &plan); err != nil || len(plan.Include) == 0 {
			t.Fatalf("invalid maximal PR plan: %v: %s", err, out)
		}
		return len(plan.Include)
	}
	var matrix map[string]any
	if err := node.Decode(&matrix); err != nil {
		t.Fatalf("job %s matrix: %v", name, err)
	}
	if include, ok := matrix["include"].([]any); ok {
		return len(include)
	}
	width := 1
	for _, values := range matrix {
		list, ok := values.([]any)
		if !ok || len(list) == 0 {
			t.Fatalf("job %s has an unreadable matrix", name)
		}
		width *= len(list)
	}
	return width
}

// readCIJobIfs returns each job's `if` and its concurrent width. A job that
// needs required-ci runs after every other job has finished, so it never
// competes with them for runners and has width zero. Workflow lint and the
// leak scan each use one runner and run in sequence, so count only the scan.
// The receipt gate precedes the full jobs and can overlap only the scan in
// these full-run scenarios, so it does not add to their concurrent peak.
func readCIJobIfs(t *testing.T) (map[string]string, map[string]int) {
	t.Helper()
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			If       string    `yaml:"if"`
			Needs    yaml.Node `yaml:"needs"`
			Strategy struct {
				Matrix yaml.Node `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	ifs := map[string]string{}
	widths := map[string]int{}
	for name, job := range workflow.Jobs {
		ifs[name] = job.If
		widths[name] = ciJobWidth(t, name, job.Strategy.Matrix)
		if name == "tree-reuse-gate" {
			widths[name] = 0
		}
		var needs []string
		if job.Needs.Kind == yaml.ScalarNode {
			needs = []string{job.Needs.Value}
		} else if job.Needs.Kind != 0 {
			if err := job.Needs.Decode(&needs); err != nil {
				t.Fatal(err)
			}
		}
		for _, need := range needs {
			if need == "required-ci" || (name == "workflow-lint" && need == "leak-scan" && job.Strategy.Matrix.Kind == 0 && workflow.Jobs["leak-scan"].If == "") {
				widths[name] = 0
			}
		}
	}
	return ifs, widths
}

// A train or release pull request gets exactly one run: the full suite as the
// pull request check, with no separate fast lane and no dispatch.
func TestCITrainPullRequestRunsFullSuiteInPlaceOfFastLane(t *testing.T) {
	ifs, _ := readCIJobIfs(t)
	push := ciScenarios[0]
	for _, sc := range ciScenarios[1:] {
		for name, expr := range ifs {
			if !strings.Contains(expr, "github.event_name != 'pull_request'") {
				continue
			}
			want := ciTrainJobAllowedOnPR(name, expr, sc)
			if got := ciIfApplies(t, expr, sc); got != want {
				t.Errorf("%s: job %s runs=%v, want %v", sc.name, name, got, want)
			}
		}
		if ciIfApplies(t, ifs["pr-tests"], sc) == (sc.headRef != "feature/example") {
			t.Errorf("%s: pr-tests runs=%v", sc.name, ciIfApplies(t, ifs["pr-tests"], sc))
		}
	}
	// Every job that runs on a push also runs for a train pull request.
	for name, expr := range ifs {
		if ciIfApplies(t, expr, push) && !ciIfApplies(t, expr, ciScenarios[2]) {
			t.Errorf("job %s runs on push but not for a train pull request", name)
		}
	}
}

func TestCITrainPullRequestStaysInsideJobBudget(t *testing.T) {
	ifs, widths := readCIJobIfs(t)
	if _, ok := ifs["pr-tests"]; !ok {
		t.Fatal("pr-tests job missing")
	}
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Needs    yaml.Node `yaml:"needs"`
			Strategy struct {
				Matrix yaml.Node `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	needs := make(map[string][]string)
	for name, job := range workflow.Jobs {
		widths[name] = ciJobWidth(t, name, job.Strategy.Matrix)
		if job.Needs.Kind == yaml.ScalarNode {
			needs[name] = []string{job.Needs.Value}
		} else if job.Needs.Kind != 0 {
			var dependencies []string
			if err := job.Needs.Decode(&dependencies); err != nil {
				t.Fatal(err)
			}
			needs[name] = dependencies
		}
	}
	count := func(sc ciScenario) int {
		active := make(map[string]int)
		for name, expr := range ifs {
			if ciIfApplies(t, expr, sc) {
				active[name] = widths[name]
			}
		}
		return ciConcurrentPeak(active, needs)
	}
	pushTotal := count(ciScenarios[0])
	for _, sc := range ciScenarios {
		total := count(sc)
		if strings.HasSuffix(sc.name, "pull_request") && strings.Contains(sc.headRef, "/") && sc.headRef != "feature/example" && total < pushTotal {
			t.Errorf("%s schedules %d jobs, fewer than the %d of a full push run", sc.name, total, pushTotal)
		}
		if total > ciPushJobBudget {
			t.Errorf("%s schedules %d jobs, budget %d", sc.name, total, ciPushJobBudget)
		}
		t.Logf("%s=%d", sc.name, total)
	}
}

// Required CI keeps one meaning for the merge rule: a train pull request must
// pass every full-suite job, and a skipped full-suite job is a failure.
func TestCIRequiredCITrainPullRequestRequiresFullSuite(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Needs yaml.Node `yaml:"needs"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["required-ci"]
	var needs []string
	if err := job.Needs.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	results := map[string]map[string]string{}
	for _, name := range needs {
		results[name] = map[string]string{"result": "success"}
	}
	run := func(headRef string, want bool) {
		t.Helper()
		encoded, _ := json.Marshal(results)
		cmd := exec.Command("bash", "-c", job.Steps[0].Run)
		cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+string(encoded), "EVENT_NAME=pull_request", "HEAD_REF="+headRef, "SMOKE_REQUIRED=false", "PR_BROWSER=false", `PR_PLAN={"include":[{"shard":-1,"packages":[],"pool":[],"browser":false}]}`, `PR_VALIDATED_PLAN={"include":[{"shard":-1,"packages":[],"pool":[],"browser":false}]}`, "PR_SELECTION=[]")
		out, err := cmd.CombinedOutput()
		if (err == nil) != want {
			t.Fatalf("head %q results %s: err=%v want pass=%v\n%s", headRef, encoded, err, want, out)
		}
	}
	// Ordinary pull requests require the site and fast-lane jobs.
	results["site"]["result"] = "skipped"
	results["pr-tests"]["result"] = "success"
	run("feature/example", false)
	results["site"]["result"] = "success"
	run("feature/example", true)
	results["site"]["result"] = "skipped"
	// Train and release pull requests: the full suite is required.
	results["pr-tests"]["result"] = "skipped"
	run("train/example", false)
	run("release/example", false)
	for _, name := range needs {
		if name != "pr-tests" {
			results[name]["result"] = "success"
		}
	}
	run("train/example", true)
	run("release/example", true)
	results["apextest"]["result"] = "failure"
	run("train/example", false)
	results["apextest"]["result"] = "success"
	results["vet"]["result"] = "failure"
	run("release/example", false)
}

func TestCIPlannedMatrixHasBoundedBudget(t *testing.T) {
	_, widths := readCIJobIfs(t)
	if widths["pr-tests"] <= 1 {
		t.Fatalf("planned PR matrix width = %d; expected the full occupied plan", widths["pr-tests"])
	}
	if widths["pr-tests"] >= ciPushJobBudget {
		t.Fatalf("planned PR matrix alone exhausts budget: %d", widths["pr-tests"])
	}
}

func TestCIIfScenariosEvaluatePRDecisions(t *testing.T) {
	for _, expr := range []string{
		"github.event_name == 'pull_request' && needs.pr-select.outputs.smoke == 'true'",
		"github.event_name == 'pull_request' && needs.pr-select.outputs.browser == 'true'",
	} {
		if ciIfApplies(t, expr, ciScenarios[0]) {
			t.Errorf("push unexpectedly matches %s", expr)
		}
		if !ciIfApplies(t, expr, ciScenarios[1]) {
			t.Errorf("PR with selection unexpectedly excludes %s", expr)
		}
	}
}

func TestCITrainScopeOnlyPermitsSelectedSmokeJobs(t *testing.T) {
	ordinaryPR := ciScenarios[1]
	expr := "github.event_name != 'pull_request' || needs.pr-select.outputs.smoke == 'true'"
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"smoke-runtime", true},
		{"smoke-distribution", true},
		{"apextest", false},
	} {
		if got := ciTrainJobAllowedOnPR(tc.name, expr, ordinaryPR); got != tc.want {
			t.Errorf("%s: allowed=%v, want %v", tc.name, got, tc.want)
		}
	}
}

// Only packaging smoke may join the fast PR route when selected.
func ciTrainJobAllowedOnPR(name, expr string, sc ciScenario) bool {
	if sc.headRef != "feature/example" {
		return true
	}
	return (name == "smoke-runtime" || name == "smoke-distribution") && strings.Contains(expr, "needs.pr-select.outputs.smoke == 'true'")
}

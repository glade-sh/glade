package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
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

func ciJobWidth(t *testing.T, name string, matrix map[string]any) int {
	t.Helper()
	if include, ok := matrix["include"].([]any); ok {
		return len(include)
	}
	width := 1
	for _, values := range matrix {
		if list, ok := values.([]any); ok {
			width *= len(list)
		}
	}
	return width
}

func readCIJobIfs(t *testing.T) (map[string]string, map[string]int) {
	t.Helper()
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			If       string `yaml:"if"`
			Strategy struct {
				Matrix map[string]any `yaml:"matrix"`
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
			want := sc.headRef != "feature/example"
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
	count := func(sc ciScenario) int {
		total := 0
		for name, expr := range ifs {
			if ciIfApplies(t, expr, sc) {
				total += widths[name]
			}
		}
		return total
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
		cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+string(encoded), "EVENT_NAME=pull_request", "HEAD_REF="+headRef)
		out, err := cmd.CombinedOutput()
		if (err == nil) != want {
			t.Fatalf("head %q results %s: err=%v want pass=%v\n%s", headRef, encoded, err, want, out)
		}
	}
	// Ordinary pull request: only the fast lane is required.
	results["site"]["result"] = "skipped"
	results["pr-tests"]["result"] = "success"
	run("feature/example", true)
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

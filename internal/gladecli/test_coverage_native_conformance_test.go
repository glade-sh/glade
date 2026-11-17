package gladecli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexcov"
	"github.com/glade-sh/glade/internal/testreport"
)

const coverageNativeDir = "../apextest/testdata/native_conformance/test_runner_coverage"

type coverageNativeRow struct {
	line                                         int
	api, kind, subject, step, covered, uncovered string
	detail                                       string
}

func readCoverageNativeRows(t *testing.T) []coverageNativeRow {
	t.Helper()
	f, err := os.Open(filepath.Join(coverageNativeDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []coverageNativeRow
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for scanner.Scan() {
		n++
		if n == 1 {
			continue
		}
		cols := strings.Split(scanner.Text(), "\t")
		if len(cols) != 8 {
			t.Fatalf("org.tsv:%d has %d columns", n, len(cols))
		}
		rows = append(rows, coverageNativeRow{line: n, api: cols[1], kind: cols[2], subject: cols[3], step: cols[4], covered: cols[5], uncovered: cols[6], detail: cols[7]})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// coverageNativeCapture is one native capture's org: the classes and
// triggers deployed (their aggregate rows open the capture), the test classes
// its steps ran, and the test class of each coverage step.
type coverageNativeCapture struct {
	prefix     string
	components []string
	tests      []string
	steps      map[string]string
}

var coverageNativeCaptures = []coverageNativeCapture{
	{
		prefix: "1:",
		components: []string{"CvBig", "CvDeclOnly", "CvEmpty", "CvException", "CvHelperPlain", "CvSetupCov", "CvSetupCovPos",
			"CvSetupStatic", "CvShapeA", "CvShapeB", "CvSmall", "CvTrig", "CvUnion", "RunnerDependency"},
		tests: []string{"CvShapeATest", "CvShapeBTest", "CvHelperUser", "CvHelperTest", "CvEmptyTest", "CvDeclOnlyTest", "CvUnionTest",
			"CvWeightTest", "CvSetupCovTest", "CvSetupCovPosTest", "CvSetupStaticTest", "CvSetupStaticNoRefTest", "CvTrigSetupTest", "CvTrigTest"},
		steps: map[string]string{
			"cv1a-": "CvShapeATest", "cv1b-": "CvShapeBTest", "cv1-helpers": "CvHelperUser", "cv8-empty": "CvEmptyTest",
			"cv8-decl": "CvDeclOnlyTest", "cv2-": "CvUnionTest", "cv4-weight": "CvWeightTest", "cv3-setup": "CvSetupCovTest",
			"cv3-setuppos": "CvSetupCovPosTest", "cv3-staticnoref": "CvSetupStaticNoRefTest", "cv3-staticref": "CvSetupStaticTest",
			"cv3-trigsetup": "CvTrigSetupTest", "cv3-trigbody": "CvTrigTest", "cv5-": "CvUnionTest",
		},
	},
	{
		prefix:     "2:",
		components: []string{"CvAsyncJob", "CvException", "CvFail", "CvShapeC", "CvTrigEmpty", "CvTrigMulti"},
		tests:      []string{"CvShapeCTest", "CvFailTest", "CvAsyncSetupTest", "CvAsyncBodyTest", "CvContactTest"},
		steps: map[string]string{
			"cv1c-": "CvShapeCTest", "cvfail-": "CvFailTest", "cvasync-setup": "CvAsyncSetupTest",
			"cvasync-body": "CvAsyncBodyTest", "cvcontact": "CvContactTest",
		},
	},
}

// coverageNativeKnownGaps lists native rows Glade does not reproduce, with
// the reason. docs/CODE_COVERAGE.md lists the same gaps.
// cv2-ab, once listed here, is an asynchronous run: those list no uncalled
// trigger (test_runner_coverage_carry f7 rows).
var coverageNativeKnownGaps = map[string]string{}

func (c coverageNativeCapture) stepClass(tag string) (string, bool) {
	if class, ok := c.steps[tag]; ok {
		return class, true
	}
	for prefix, class := range c.steps {
		if strings.HasSuffix(prefix, "-") && strings.HasPrefix(tag, prefix) {
			return class, true
		}
	}
	return "", false
}

func writeCoverageNativeProject(t *testing.T, api string, capture coverageNativeCapture) string {
	t.Helper()
	root := t.TempDir()
	writeDaemonEquivalenceFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for _, name := range append(append([]string(nil), capture.components...), capture.tests...) {
		if source, err := os.ReadFile(filepath.Join(coverageNativeDir, "fixtures", name+".cls")); err == nil {
			path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
			writeDaemonEquivalenceFile(t, path, string(source))
			writeDaemonEquivalenceFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			continue
		}
		source, err := os.ReadFile(filepath.Join(coverageNativeDir, "fixtures", name+".trigger"))
		if err != nil {
			t.Fatalf("fixture %s: %v", name, err)
		}
		path := filepath.Join(root, "force-app/main/default/triggers", name+".trigger")
		writeDaemonEquivalenceFile(t, path, string(source))
		writeDaemonEquivalenceFile(t, path+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
	}
	return root
}

// runCoverageCLI runs glade test locally and returns the JSON envelope's run,
// the raw stdout and the exit code.
func runCoverageCLI(t *testing.T, root string, args ...string) (testreport.Run, string, int) {
	t.Helper()
	all := append([]string{"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache"}, args...)
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), all, &stdout, &stderr)
	var envelope struct {
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Data     testreport.Run     `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("glade %s: decode JSON: %v\nstdout=%s\nstderr=%s", strings.Join(all, " "), err, stdout.String(), stderr.String())
	}
	if envelope.ExitCode != exit {
		t.Fatalf("envelope exit=%d process exit=%d", envelope.ExitCode, exit)
	}
	return envelope.Data, stdout.String(), exit
}

func coverageLineList(t *testing.T, s string) []int {
	t.Helper()
	if s == "" {
		return []int{}
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		v, err := strconv.Atoi(part)
		if err != nil {
			t.Fatalf("line list %q: %v", s, err)
		}
		out = append(out, v)
	}
	return out
}

func coverageLinesText(lines []int) string {
	parts := make([]string, len(lines))
	for i, line := range lines {
		parts[i] = strconv.Itoa(line)
	}
	return strings.Join(parts, ",")
}

var coverageRunDetail = regexp.MustCompile(`ran=(\d+) .*testRunCoverage=(\d+)% orgWide=(\d+)% classes=(\S+)`)
var coverageRunClass = regexp.MustCompile(`^(\w+):(\d+)/(\d+)=(\d+)$`)

type coverageNativeClassRow struct {
	name                    string
	covered, total, percent int
}

func parseCoverageRunClasses(t *testing.T, row coverageNativeRow, list string) []coverageNativeClassRow {
	t.Helper()
	var out []coverageNativeClassRow
	for _, item := range strings.Split(list, ";") {
		m := coverageRunClass.FindStringSubmatch(item)
		if m == nil {
			t.Fatalf("org.tsv:%d class item %q", row.line, item)
		}
		covered, _ := strconv.Atoi(m[2])
		total, _ := strconv.Atoi(m[3])
		percent, _ := strconv.Atoi(m[4])
		out = append(out, coverageNativeClassRow{m[1], covered, total, percent})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// NATIVE glade test --code-coverage replays the captured coverage timeline,
// test_runner_coverage/org.tsv agg, per, org and run rows, API 62.0 and 67.0:
// per-test covered and uncovered lines (solo runs), aggregates that only
// grow, per-test rows replaced per test class, ApexOrgWideCoverage floored,
// the sf run summary rounded half up and weighted by lines. glade test
// reports one run against a fresh org; the replay feeds its per-test rows
// into apexcov.Store in the native step order.
func TestCodeCoverageNativeConformance(t *testing.T) {
	if testing.Short() {
		t.Skip("runs glade test over the native coverage fixtures")
	}
	rows := readCoverageNativeRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		for _, capture := range coverageNativeCaptures {
			t.Run(api+"/capture"+strings.TrimSuffix(capture.prefix, ":"), func(t *testing.T) {
				root := writeCoverageNativeProject(t, api, capture)
				run, _, _ := runCoverageCLI(t, root, "--code-coverage")
				if run.Coverage == nil {
					t.Fatal("run has no coverage")
				}
				// Executable sets: glade lists every component a test reached
				// and every trigger; each must equal the native set.
				executable := map[string][]int{}
				for _, c := range run.Coverage.Components {
					lines := append(append([]int(nil), c.CoveredLines...), c.UncoveredLines...)
					sort.Ints(lines)
					executable[c.Name] = lines
				}
				nativeExec := map[string][]int{}
				for _, r := range rows {
					if r.api == api && r.kind == "agg" && r.step == capture.prefix+"00-empty" {
						lines := append(coverageLineList(t, r.covered), coverageLineList(t, r.uncovered)...)
						sort.Ints(lines)
						nativeExec[r.subject] = lines
						if got, ok := executable[r.subject]; ok && coverageLinesText(got) != coverageLinesText(lines) {
							t.Errorf("org.tsv:%d %s executable lines got %v want %v", r.line, r.subject, got, lines)
						}
					}
				}
				store := apexcov.NewStore(nativeExec)
				var steps []string
				seen := map[string]bool{}
				for _, r := range rows {
					if r.api == api && r.kind == "org" && strings.HasPrefix(r.step, capture.prefix) && !seen[r.step] {
						seen[r.step] = true
						steps = append(steps, r.step)
					}
				}
				checked := map[string]int{}
				for _, step := range steps {
					tag := strings.TrimPrefix(step, capture.prefix)
					if tag == "00-empty" || tag == "final" {
						continue
					}
					testClass, ok := capture.stepClass(tag)
					if !ok {
						t.Fatalf("step %s has no test class", step)
					}
					var runRow *coverageNativeRow
					var methods []string
					async := false
					for i := range rows {
						r := rows[i]
						if r.api != api || r.subject != tag {
							continue
						}
						switch r.kind {
						case "run":
							runRow = &rows[i]
						case "test":
							methods = append(methods, strings.SplitN(r.detail, "|", 2)[0])
							// An asynchronous run's rows read None for an empty
							// message; a synchronous run's read "".
							if strings.HasSuffix(r.detail, "|None|None") {
								async = true
							}
						}
					}
					if runRow == nil || len(methods) == 0 {
						t.Fatalf("step %s has no run or test rows", step)
					}
					if gap, ok := coverageNativeKnownGaps[step]; ok {
						t.Logf("%s: known gap: %s", step, gap)
						continue
					}
					m := coverageRunDetail.FindStringSubmatch(runRow.detail)
					if m == nil {
						// A run without --code-coverage records nothing new.
						checked["nocov"]++
						continue
					}
					// Each step is its own glade run of the step's selection:
					// per-test rows carry declaration lines between the run's
					// tests, as the native rows of that step do.
					args := []string{"--code-coverage", "--class", testClass}
					if !async {
						args = append(args, "--synchronous")
					}
					if len(methods) == 1 {
						args = append(args, "--method", methods[0])
					} else {
						var names []string
						for _, method := range methods {
							names = append(names, testClass+"."+method)
						}
						args = append(args, "--filter", strings.Join(names, ","))
					}
					stepRun, _, _ := runCoverageCLI(t, root, args...)
					if stepRun.Coverage == nil {
						t.Fatalf("step %s: no coverage", step)
					}
					stepRows := map[string]map[string][]int{}
					for _, suite := range stepRun.Suites {
						for _, c := range suite.Cases {
							stepRows[c.ClassName+"."+c.MethodName] = map[string][]int{}
							for _, row := range c.Coverage {
								stepRows[c.ClassName+"."+c.MethodName][row.Name] = row.CoveredLines
							}
						}
					}
					var runs []apexcov.TestRun
					runListed := map[string]bool{}
					for _, c := range stepRun.Coverage.Components {
						runListed[c.Name] = true
					}
					for _, method := range methods {
						key := testClass + "." + method
						covered, ok := stepRows[key]
						if !ok {
							t.Fatalf("step %s: glade ran no %s", step, key)
						}
						runs = append(runs, apexcov.TestRun{TestClass: testClass, Method: method, Covered: covered})
					}
					if len(stepRows) != len(methods) {
						t.Fatalf("step %s: glade ran %d tests, native %d", step, len(stepRows), len(methods))
					}
					store.Record(runs)
					for _, r := range rows {
						if r.api != api || r.step != step {
							continue
						}
						switch r.kind {
						case "agg":
							got := store.Aggregate(r.subject)
							if coverageLinesText(got.Covered) != r.covered || coverageLinesText(got.Uncovered) != r.uncovered {
								t.Errorf("org.tsv:%d aggregate %s after %s\n got covered %v uncovered %v\nwant covered %s uncovered %s", r.line, r.subject, step, got.Covered, got.Uncovered, r.covered, r.uncovered)
							}
							checked["agg"]++
						case "org":
							if got := strconv.Itoa(store.OrgWide()); got != r.detail {
								t.Errorf("org.tsv:%d org-wide after %s got %s want %s", r.line, step, got, r.detail)
							}
							checked["org"]++
						case "per":
							class, method, _ := strings.Cut(r.subject, ".")
							got, ok := store.PerTest(class, method, r.detail)
							if !ok {
								t.Errorf("org.tsv:%d %s has no %s row", r.line, r.subject, r.detail)
								continue
							}
							covered := map[int]bool{}
							for _, line := range got {
								covered[line] = true
							}
							var uncovered []int
							for _, line := range nativeExec[r.detail] {
								if !covered[line] {
									uncovered = append(uncovered, line)
								}
							}
							if coverageLinesText(got) != r.covered || coverageLinesText(uncovered) != r.uncovered {
								t.Errorf("org.tsv:%d %s/%s\n got covered %v uncovered %v\nwant covered %s uncovered %s", r.line, r.subject, r.detail, got, uncovered, r.covered, r.uncovered)
							}
							checked["per"]++
						}
					}
					if _, gap := coverageNativeKnownGaps[step+"/run"]; gap {
						continue
					}
					want := parseCoverageRunClasses(t, *runRow, m[4])
					var parts []apexcov.Counts
					var got []coverageNativeClassRow
					for name := range runListed {
						count := store.Aggregate(name).Of()
						parts = append(parts, count)
						got = append(got, coverageNativeClassRow{name, count.Covered, count.Total, apexcov.Percent(count.Covered, count.Total)})
					}
					sort.Slice(got, func(i, j int) bool { return got[i].name < got[j].name })
					if fmt.Sprint(got) != fmt.Sprint(want) {
						t.Errorf("org.tsv:%d run %s classes\n got %v\nwant %v", runRow.line, tag, got, want)
					}
					if pct := strconv.Itoa(apexcov.RunPercent(parts)); pct != m[2] {
						t.Errorf("org.tsv:%d run %s testRunCoverage got %s want %s", runRow.line, tag, pct, m[2])
					}
					if org := strconv.Itoa(store.OrgWide()); org != m[3] {
						t.Errorf("org.tsv:%d run %s orgWide got %s want %s", runRow.line, tag, org, m[3])
					}
					checked["run"]++
				}
				t.Logf("checked %v", checked)
				if checked["agg"] == 0 || checked["org"] == 0 || checked["per"] == 0 || checked["run"] == 0 {
					t.Fatalf("checked %v", checked)
				}
			})
		}
	}
}

// NATIVE a fresh-org run's own summary: glade test --code-coverage for the
// first coverage step of each capture and for each whole-class run matches
// the sf run summary, test_runner_coverage/org.tsv run rows cv1a-leftBranch,
// cv1a-all, cv1b-all, cv1c-one, cv1c-all, API 62.0 and 67.0. Org-wide is
// compared only where no earlier step covered another class.
func TestCodeCoverageRunSummaryNative(t *testing.T) {
	if testing.Short() {
		t.Skip("runs glade test over the native coverage fixtures")
	}
	rows := readCoverageNativeRows(t)
	cases := []struct {
		capture int
		tag     string
		args    []string
		orgWide bool
	}{
		{0, "cv1a-leftBranch", []string{"--class", "CvShapeATest", "--method", "leftBranch"}, true},
		{0, "cv1a-all", []string{"--class", "CvShapeATest"}, true},
		{0, "cv1b-all", []string{"--class", "CvShapeBTest"}, false},
		{1, "cv1c-one", []string{"--class", "CvShapeCTest", "--method", "one"}, true},
		{1, "cv1c-all", []string{"--class", "CvShapeCTest"}, true},
	}
	for _, api := range []string{"62.0", "67.0"} {
		roots := map[int]string{}
		for i, capture := range coverageNativeCaptures {
			roots[i] = writeCoverageNativeProject(t, api, capture)
		}
		for _, tc := range cases {
			t.Run(api+"/"+tc.tag, func(t *testing.T) {
				root := roots[tc.capture]
				var native *coverageNativeRow
				for i := range rows {
					if rows[i].api == api && rows[i].kind == "run" && rows[i].subject == tc.tag {
						native = &rows[i]
					}
				}
				if native == nil {
					t.Fatalf("no native run row %s", tc.tag)
				}
				m := coverageRunDetail.FindStringSubmatch(native.detail)
				run, _, exit := runCoverageCLI(t, root, append([]string{"--code-coverage", "--synchronous"}, tc.args...)...)
				if exit != 0 || run.Coverage == nil {
					t.Fatalf("exit=%d coverage=%v", exit, run.Coverage)
				}
				var got []coverageNativeClassRow
				for _, c := range run.Coverage.Components {
					got = append(got, coverageNativeClassRow{c.Name, c.TotalCovered, c.TotalLines, c.CoveredPercent})
				}
				sort.Slice(got, func(i, j int) bool { return got[i].name < got[j].name })
				if want := parseCoverageRunClasses(t, *native, m[4]); fmt.Sprint(got) != fmt.Sprint(want) {
					t.Errorf("org.tsv:%d classes got %v want %v", native.line, got, want)
				}
				if got := strconv.Itoa(run.Coverage.Summary.TestRunCoverage); got != m[2] {
					t.Errorf("org.tsv:%d testRunCoverage got %s want %s", native.line, got, m[2])
				}
				if got := strconv.Itoa(run.Coverage.Summary.OrgWideCoverage); tc.orgWide && got != m[3] {
					t.Errorf("org.tsv:%d orgWideCoverage got %s want %s", native.line, got, m[3])
				}
			})
		}
	}
}

// A run without --code-coverage has no coverage block, as sf apex run test
// without --code-coverage has none (org.tsv run row cv5-nocov).
func TestCodeCoverageOffAddsNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("runs glade test over the native coverage fixtures")
	}
	root := writeCoverageNativeProject(t, "67.0", coverageNativeCaptures[0])
	run, stdout, _ := runCoverageCLI(t, root, "--class", "CvUnionTest")
	if run.Coverage != nil || strings.Contains(stdout, `"coverage"`) {
		t.Fatalf("coverage reported without --code-coverage:\n%s", stdout)
	}
	var console bytes.Buffer
	if err := testreport.WriteConsole(&console, run); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(console.String(), "Code coverage") {
		t.Fatalf("console shows coverage without --code-coverage:\n%s", console.String())
	}
}

// The local route, the in-process daemon and a real test server report the
// same coverage, the same failed --coverage-gate and the same exit code. The
// managed package dependency is not a coverage component.
func TestDaemonEquivalenceCodeCoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("infrastructure test; full suite runs in acceptance lanes")
	}
	scenario := daemonEquivalenceScenario{
		name: "code_coverage",
		classes: map[string]string{
			"Shape": `public class Shape {
    public static Integer pick(Boolean a, Boolean b) {
        return a
            && b
            ? 1
            : 2;
    }
    public static Integer unused() {
        return 3;
    }
}`,
			"ShapeTest": `@IsTest private class ShapeTest {
    @IsTest static void left() { System.assertEquals(2, Shape.pick(false, true)); }
}`,
		},
		args:               []string{"--code-coverage", "--coverage-gate", "RunLocalTests"},
		wantExit:           1,
		wantTotal:          1,
		wantPass:           1,
		wantEnvelopeStatus: "failed",
	}
	modes := []daemonEquivalenceMode{
		{name: "local", args: []string{"--no-serve"}},
		{name: "in_process_daemon", args: []string{"--daemon"}},
		{name: "auto_connect", realServer: true},
		{name: "explicit_connect", args: []string{"--connect"}, realServer: true},
	}
	oracle := runDaemonEquivalenceInvocation(t, scenario, modes[0])
	assertDaemonEquivalenceScenario(t, scenario, modes[0], oracle)
	coverage := oracle.Run.Coverage
	if coverage == nil || coverage.Gate == nil || coverage.Gate.Passed || !oracle.Summary.CoverageGateFailed {
		t.Fatalf("local coverage = %+v", coverage)
	}
	if len(coverage.Components) != 1 || coverage.Components[0].Name != "Shape" ||
		fmt.Sprint(coverage.Components[0].CoveredLines) != "[2 3 6]" || fmt.Sprint(coverage.Components[0].UncoveredLines) != "[4 5 8 9]" {
		t.Fatalf("local components = %+v", coverage.Components)
	}
	if coverage.Summary.TestRunCoverage != 43 || coverage.Summary.OrgWideCoverage != 42 || coverage.Gate.Percent != 42 {
		t.Fatalf("local summary = %+v gate = %+v", coverage.Summary, coverage.Gate)
	}
	for _, mode := range modes[1:] {
		got := runDaemonEquivalenceInvocation(t, scenario, mode)
		assertDaemonEquivalenceScenario(t, scenario, mode, got)
		if !reflect.DeepEqual(got, oracle) {
			t.Errorf("%s diverged from local oracle:\nwant %s\n got %s", mode.name, formatDaemonEquivalenceSnapshot(oracle), formatDaemonEquivalenceSnapshot(got))
		}
	}
}

const coverageSuiteDir = "../apextest/testdata/native_conformance/test_runner_coverage_suite"

// NATIVE per-test carry-over and trigger rows, test_runner_coverage_suite/org.tsv,
// API 62.0 and 67.0, repeats r1 and r2: one synchronous run of CovSuiteTest
// (14 methods). glade test --code-coverage --json reports every native
// per-test row (115 per run) and every aggregate row with the same covered
// and uncovered lines.
func TestCodeCoverageSuiteNative(t *testing.T) {
	if testing.Short() {
		t.Skip("runs glade test over the native coverage fixtures")
	}
	f, err := os.Open(filepath.Join(coverageSuiteDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type row struct {
		line                              int
		api, repeat, source, name, method string
		covered, uncovered                string
	}
	var rows []row
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		n++
		if n == 1 {
			continue
		}
		cols := strings.Split(scanner.Text(), "\t")
		if len(cols) != 9 {
			t.Fatalf("org.tsv:%d has %d columns", n, len(cols))
		}
		rows = append(rows, row{n, cols[0], cols[1], cols[2], cols[3], cols[4], cols[7], cols[8]})
	}
	entries, err := os.ReadDir(filepath.Join(coverageSuiteDir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeDaemonEquivalenceFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, e := range entries {
				source, err := os.ReadFile(filepath.Join(coverageSuiteDir, "fixtures", e.Name()))
				if err != nil {
					t.Fatal(err)
				}
				dir, meta := "classes", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>"
				if strings.HasSuffix(e.Name(), ".trigger") {
					dir, meta = "triggers", "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>"
				}
				path := filepath.Join(root, "force-app/main/default", dir, e.Name())
				writeDaemonEquivalenceFile(t, path, string(source))
				writeDaemonEquivalenceFile(t, path+"-meta.xml", meta)
			}
			run, _, _ := runCoverageCLI(t, root, "--code-coverage", "--synchronous", "--class", "CovSuiteTest")
			if run.Coverage == nil {
				t.Fatal("no coverage")
			}
			lines := map[string]testreport.ComponentCoverage{}
			for _, c := range run.Coverage.Components {
				lines[c.Name] = c
			}
			perTest := map[string][]int{}
			listed := map[string]bool{}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					for _, r := range c.Coverage {
						perTest[c.MethodName+"/"+r.Name] = r.CoveredLines
						listed[c.MethodName+"/"+r.Name] = true
					}
				}
			}
			checked, nativeRows := 0, map[string]bool{}
			for _, r := range rows {
				if r.api != api {
					continue
				}
				switch r.source {
				case "aggregate":
					c, ok := lines[r.name]
					if !ok {
						// An untouched empty class: the Tooling aggregate lists it
						// at 0/0; glade lists only what the run reached.
						if r.covered != "" || r.uncovered != "" {
							t.Errorf("org.tsv:%d %s missing from glade components", r.line, r.name)
						}
						continue
					}
					if coverageLinesText(c.CoveredLines) != r.covered || coverageLinesText(c.UncoveredLines) != r.uncovered {
						t.Errorf("org.tsv:%d aggregate %s\n got %v / %v\nwant %s / %s", r.line, r.name, c.CoveredLines, c.UncoveredLines, r.covered, r.uncovered)
					}
				case "per-test":
					key := r.method + "/" + r.name
					nativeRows[key] = true
					if !listed[key] {
						t.Errorf("org.tsv:%d no glade row %s", r.line, key)
						continue
					}
					got := perTest[key]
					covered := map[int]bool{}
					for _, line := range got {
						covered[line] = true
					}
					var uncovered []int
					for _, line := range append(append([]int(nil), lines[r.name].CoveredLines...), lines[r.name].UncoveredLines...) {
						if !covered[line] {
							uncovered = append(uncovered, line)
						}
					}
					sort.Ints(uncovered)
					if coverageLinesText(got) != r.covered || coverageLinesText(uncovered) != r.uncovered {
						t.Errorf("org.tsv:%d %s\n got %v / %v\nwant %s / %s", r.line, key, got, uncovered, r.covered, r.uncovered)
					}
				}
				checked++
			}
			for key := range listed {
				if !nativeRows[key] {
					t.Errorf("glade row %s has no native row", key)
				}
			}
			t.Logf("checked %d rows", checked)
		})
	}
}

// The RunLocalTests rule is org-wide, so it refuses a partial selection;
// RunSpecifiedTests applies to one.
func TestCoverageGateRunLocalTestsRejectsPartialSelections(t *testing.T) {
	root := writeCoverageNativeProject(t, "67.0", coverageNativeCaptures[0])
	for _, args := range [][]string{
		{"--class", "CvUnionTest"}, {"--filter", "CvUnion"}, {"--shard-count", "2", "--shard-index", "0"}, {"--changed-since", "HEAD"},
	} {
		all := append([]string{"test", "--project", root, "--no-serve", "--no-progress", "--coverage-gate", "RunLocalTests"}, args...)
		var stdout, stderr bytes.Buffer
		exit := Run(context.Background(), all, &stdout, &stderr)
		if exit == 0 || !strings.Contains(stdout.String()+stderr.String(), "--coverage-gate RunLocalTests needs every local test") {
			t.Fatalf("%v: exit=%d stdout=%s stderr=%s", args, exit, stdout.String(), stderr.String())
		}
	}
	run, _, _ := runCoverageCLI(t, root, "--coverage-gate", "RunSpecifiedTests", "--class", "CvUnionTest")
	if run.Coverage == nil || run.Coverage.Gate == nil || run.Coverage.Gate.TestLevel != "RunSpecifiedTests" {
		t.Fatalf("RunSpecifiedTests with a selector: %+v", run.Coverage)
	}
}

const coverageCarryDir = "../apextest/testdata/native_conformance/test_runner_coverage_carry"

func writeCoverageCarryProject(t *testing.T, api, set string) string {
	t.Helper()
	root := t.TempDir()
	writeDaemonEquivalenceFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	entries, err := os.ReadDir(filepath.Join(coverageCarryDir, "fixtures", set))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		source, err := os.ReadFile(filepath.Join(coverageCarryDir, "fixtures", set, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		dir, meta := "classes", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>"
		if strings.HasSuffix(e.Name(), ".trigger") {
			dir, meta = "triggers", "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>"
		}
		path := filepath.Join(root, "force-app/main/default", dir, e.Name())
		writeDaemonEquivalenceFile(t, path, string(source))
		writeDaemonEquivalenceFile(t, path+"-meta.xml", meta)
	}
	return root
}

// NATIVE per-test carry order and scope, @TestSetup coverage, async work
// queued in @TestSetup, and trigger rows in synchronous and asynchronous runs,
// test_runner_coverage_carry/org.tsv per and test rows, API 62.0 and 67.0,
// repeats r1 and r2. Each native step is one glade run of the same selection.
func TestCodeCoverageCarryNative(t *testing.T) {
	if testing.Short() {
		t.Skip("runs glade test over the native coverage fixtures")
	}
	f, err := os.Open(filepath.Join(coverageCarryDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type row struct {
		line                                                     int
		api, repeat, step, kind, class, method, component, cover string
		detail                                                   string
	}
	var rows []row
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for scanner.Scan() {
		n++
		if n == 1 {
			continue
		}
		c := strings.Split(scanner.Text(), "\t")
		if len(c) != 9 {
			t.Fatalf("org.tsv:%d has %d columns", n, len(c))
		}
		rows = append(rows, row{n, c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8]})
	}
	setupClasses := []string{"SetupQTest", "SetupTwoQTest", "SetupChainTest", "SetupFutureTest", "SetupBatchTest", "SetupThrowTest", "SetupStopTest", "SetupNoneTest"}
	steps := []struct {
		name, set string
		args      []string
	}{
		{"mix-full", "main", []string{"--class", "CarryMixTest", "--synchronous"}},
		{"mix-gamma", "main", []string{"--class", "CarryMixTest", "--method", "Gamma", "--synchronous"}},
		{"mix-full-async", "main", []string{"--class", "CarryMixTest"}},
		{"setup", "main", []string{"--class", "CarrySetupTest", "--synchronous"}},
		{"x12-par-on", "main", []string{"--filter", "CarryX1Test,CarryX2Test"}},
		{"x12-par-off", "main", []string{"--filter", "CarryX1Test,CarryX2Test"}},
		{"setup-all-async", "main", []string{"--filter", strings.Join(setupClasses, ",")}},
		{"f7a-ab-async", "f7", []string{"--filter", "CvUnionTest.aLeft,CvUnionTest.bRight"}},
		{"f7a-ab-sync", "f7", []string{"--filter", "CvUnionTest.aLeft,CvUnionTest.bRight", "--synchronous"}},
		{"f7b-a-sync", "f7", []string{"--class", "CvUnionTest", "--method", "aLeft", "--synchronous"}},
		{"f7b-ab-async", "f7", []string{"--filter", "CvUnionTest.aLeft,CvUnionTest.bRight"}},
	}
	for _, class := range setupClasses {
		steps = append(steps, struct {
			name, set string
			args      []string
		}{class, "main", []string{"--class", class, "--synchronous"}})
	}
	facts := regexp.MustCompile(`FACTS .*`)
	for _, api := range []string{"62.0", "67.0"} {
		roots := map[string]string{"main": writeCoverageCarryProject(t, api, "main"), "f7": writeCoverageCarryProject(t, api, "f7")}
		for _, step := range steps {
			t.Run(api+"/"+step.name, func(t *testing.T) {
				run, _, _ := runCoverageCLI(t, roots[step.set], append([]string{"--code-coverage"}, step.args...)...)
				got := map[string]string{}
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						for _, r := range c.Coverage {
							got[c.ClassName+"."+c.MethodName+"/"+r.Name] = coverageLinesText(r.CoveredLines)
						}
						status := "Pass"
						message := ""
						if c.Problem != nil {
							status = "Fail"
							message = c.Problem.Type + ": " + c.Problem.ResultMessage()
						}
						got[c.ClassName+"."+c.MethodName+"#test"] = status + "|" + message
					}
				}
				checked := 0
				for _, repeat := range []string{"r1", "r2"} {
					wantRows := map[string]bool{}
					for _, r := range rows {
						if r.api != api || r.repeat != repeat || r.step != step.name {
							continue
						}
						switch r.kind {
						case "per":
							key := r.class + "." + r.method + "/" + r.component
							wantRows[key] = true
							if have, ok := got[key]; !ok || have != r.cover {
								t.Errorf("org.tsv:%d %s covered got %q (listed %v) want %q", r.line, key, have, ok, r.cover)
							}
						case "test":
							key := r.class + "." + r.method + "#test"
							want, have := r.detail, got[key]
							if m := facts.FindString(want); m != "" {
								want, have = m, facts.FindString(have)
							}
							if !strings.HasPrefix(want, "Pass|") && strings.Contains(want, "|") {
								want = strings.SplitN(want, "|", 2)[1]
								have = strings.TrimPrefix(strings.TrimPrefix(have, "Fail|"), "Pass|")
							}
							if have != want && !strings.HasSuffix(have, want) {
								t.Errorf("org.tsv:%d %s outcome got %q want %q", r.line, key, have, want)
							}
						default:
							continue
						}
						checked++
					}
					if len(wantRows) == 0 {
						continue
					}
					for key := range got {
						if strings.Contains(key, "/") && !wantRows[key] {
							t.Errorf("%s %s: glade row %s has no native row", step.name, repeat, key)
						}
					}
				}
				if checked == 0 && !(api == "62.0" && step.name == "f7a-ab-async") {
					t.Fatalf("no native rows checked for %s", step.name)
				}
			})
		}
	}
}

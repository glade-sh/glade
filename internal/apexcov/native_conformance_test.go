package apexcov

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const nativeDir = "../apextest/testdata/native_conformance/test_runner_coverage"

type nativeRow struct {
	line                       int
	api, kind, subject, step   string
	covered, uncovered, detail string
}

func readNativeRows(t *testing.T) []nativeRow {
	t.Helper()
	f, err := os.Open(filepath.Join(nativeDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []nativeRow
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
		rows = append(rows, nativeRow{line: n, api: cols[1], kind: cols[2], subject: cols[3], step: cols[4], covered: cols[5], uncovered: cols[6], detail: cols[7]})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func parseLines(t *testing.T, s string) []int {
	t.Helper()
	if s == "" {
		return nil
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

func fixtureSource(t *testing.T, name string) (string, string) {
	t.Helper()
	for _, ext := range []string{".cls", ".trigger"} {
		path := filepath.Join(nativeDir, "fixtures", name+ext)
		if b, err := os.ReadFile(path); err == nil {
			return path, string(b)
		}
	}
	return "", ""
}

// stateAt replays agg rows for one API up to and including step order index.
type aggTimeline struct {
	steps []string
	state map[string]map[string]Lines // step -> class -> lines
}

func buildTimeline(t *testing.T, rows []nativeRow, api string) aggTimeline {
	t.Helper()
	tl := aggTimeline{state: map[string]map[string]Lines{}}
	cur := map[string]Lines{}
	seen := map[string]bool{}
	var lastStep string
	flush := func() {
		if lastStep == "" {
			return
		}
		snap := make(map[string]Lines, len(cur))
		for k, v := range cur {
			snap[k] = v
		}
		tl.state[lastStep] = snap
	}
	// Rows within a step are contiguous; org rows close each step.
	for _, r := range rows {
		if r.api != api {
			continue
		}
		if (r.kind == "agg" || r.kind == "org") && lastStep != "" && r.step[:1] != lastStep[:1] {
			// A new capture starts in a fresh org.
			flush()
			lastStep = ""
			cur = map[string]Lines{}
		}
		switch r.kind {
		case "agg":
			if r.step != lastStep {
				flush()
				lastStep = r.step
				if !seen[r.step] {
					tl.steps = append(tl.steps, r.step)
					seen[r.step] = true
				}
			}
			cur[r.subject] = Lines{Covered: parseLines(t, r.covered), Uncovered: parseLines(t, r.uncovered)}
		case "org":
			if r.step != lastStep {
				flush()
				lastStep = r.step
				if !seen[r.step] {
					tl.steps = append(tl.steps, r.step)
					seen[r.step] = true
				}
			}
		}
	}
	flush()
	return tl
}

// NATIVE executable-line map test_runner_coverage/org.tsv agg rows, API 62.0 and 67.0
func TestExecutableLinesNative(t *testing.T) {
	rows := readNativeRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		first := map[string]nativeRow{}
		for _, r := range rows {
			if r.api == api && r.kind == "agg" {
				if _, ok := first[r.subject]; !ok {
					first[r.subject] = r
				}
			}
		}
		if len(first) < 18 {
			t.Fatalf("api %s: only %d components with rows", api, len(first))
		}
		names := make([]string, 0, len(first))
		for name := range first {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			r := first[name]
			path, source := fixtureSource(t, name)
			if source == "" {
				continue // redeployed or unowned component; no fixture to map
			}
			want := append(parseLines(t, r.covered), parseLines(t, r.uncovered)...)
			sort.Ints(want)
			t.Run(api+"/"+name, func(t *testing.T) {
				got := ExecutableLines(path, source)
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("org.tsv:%d executable lines\n got %v\nwant %v", r.line, got, want)
				}
			})
		}
	}
}

// NATIVE test classes are excluded: no aggregate row exists for an @IsTest class.
func TestTestClassesHaveNoCoverageNative(t *testing.T) {
	rows := readNativeRows(t)
	aggregated := map[string]bool{}
	for _, r := range rows {
		if r.kind == "agg" {
			aggregated[r.subject] = true
		}
	}
	entries, err := os.ReadDir(filepath.Join(nativeDir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	istest := regexp.MustCompile(`(?is)^\s*@IsTest`)
	checked := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		path, source := fixtureSource(t, name)
		if !istest.MatchString(source) {
			continue
		}
		checked++
		if aggregated[name] {
			t.Errorf("%s is a test class with a native aggregate row", name)
		}
		if got := ExecutableLines(path, source); got != nil {
			t.Errorf("%s: test class has executable lines %v", name, got)
		}
	}
	if checked < 15 {
		t.Fatalf("checked %d test classes", checked)
	}
}

// NATIVE aggregate rows are the union of distinct covered lines across runs.
func TestUnionNative(t *testing.T) {
	rows := readNativeRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		tl := buildTimeline(t, rows, api)
		final := func(class string) Lines {
			var out Lines
			for _, step := range tl.steps {
				if l, ok := tl.state[step][class]; ok {
					out = l
				}
			}
			return out
		}
		// Solo per-test rows, one per method, in the step each method ran alone.
		solo := func(testClass string, methods []string, steps []string, target string) [][]int {
			var runs [][]int
			for i, m := range methods {
				for _, r := range rows {
					if r.api == api && r.kind == "per" && r.step == steps[i] && r.subject == testClass+"."+m && r.detail == target {
						runs = append(runs, parseLines(t, r.covered))
					}
				}
			}
			return runs
		}
		cases := []struct {
			name    string
			class   string
			testCls string
			methods []string
			steps   []string
		}{
			{"union", "CvUnion", "CvUnionTest", []string{"aLeft", "bRight"}, []string{"1:cv2-aLeft", "1:cv2-bRight"}},
			{"shapeA", "CvShapeA", "CvShapeATest", []string{"leftBranch", "rightBranch", "objects", "partiallyCompletedLine", "debugSideEffect"},
				[]string{"1:cv1a-leftBranch", "1:cv1a-rightBranch", "1:cv1a-objects", "1:cv1a-partiallyCompletedLine", "1:cv1a-debugSideEffect"}},
			{"shapeB", "CvShapeB", "CvShapeBTest", []string{"loopsAndSwitch", "guards", "objects", "constantOnly"},
				[]string{"1:cv1b-loopsAndSwitch", "1:cv1b-guards", "1:cv1b-objects", "1:cv1b-constantOnly"}},
		}
		for _, c := range cases {
			t.Run(api+"/"+c.name, func(t *testing.T) {
				runs := solo(c.testCls, c.methods, c.steps, c.class)
				if len(runs) != len(c.methods) {
					t.Fatalf("found %d solo rows for %d methods", len(runs), len(c.methods))
				}
				path, source := fixtureSource(t, c.class)
				exec := ExecutableLines(path, source)
				got := Union(exec, runs...)
				want := final(c.class)
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("union of solo runs\n got %+v\nwant %+v", got, want)
				}
				// A line hit by two tests is one covered line.
				doubled := Union(exec, append(runs, runs...)...)
				if fmt.Sprint(doubled) != fmt.Sprint(got) {
					t.Fatalf("overlapping runs inflated coverage: %+v", doubled)
				}
			})
		}
	}
}

var cliClass = regexp.MustCompile(`^(\w+):(\d+)/(\d+)=(\d+)$`)

// NATIVE percentages: ApexOrgWideCoverage floors, the sf run summary rounds
// half up, both weight by lines rather than averaging classes.
func TestPercentNative(t *testing.T) {
	rows := readNativeRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		tl := buildTimeline(t, rows, api)
		checked := 0
		for _, r := range rows {
			if r.api != api || r.kind != "org" || r.subject != "-" {
				continue
			}
			state, ok := tl.state[r.step]
			if !ok {
				continue
			}
			var parts []Counts
			var means float64
			for _, l := range state {
				parts = append(parts, l.Of())
				if l.Total() > 0 {
					means += 100 * float64(len(l.Covered)) / float64(l.Total())
				}
			}
			want, _ := strconv.Atoi(r.detail)
			if got := OrgWidePercent(parts); got != want {
				t.Errorf("org.tsv:%d step %s: OrgWidePercent=%d want %d", r.line, r.step, got, want)
			}
			checked++
			// The class-percentage mean is a different quantity and must not stand in.
			_ = means
		}
		if checked < 20 {
			t.Fatalf("api %s: only %d org rows", api, checked)
		}
	}
	runRE := regexp.MustCompile(`testRunCoverage=(\d+)% orgWide=(\d+)% classes=(\S+)`)
	cliChecked := 0
	for _, r := range rows {
		if r.kind != "run" {
			continue
		}
		m := runRE.FindStringSubmatch(r.detail)
		if m == nil {
			continue
		}
		var parts []Counts
		for _, item := range strings.Split(m[3], ";") {
			c := cliClass.FindStringSubmatch(item)
			if c == nil {
				t.Fatalf("org.tsv:%d class item %q", r.line, item)
			}
			covered, _ := strconv.Atoi(c[2])
			total, _ := strconv.Atoi(c[3])
			wantPct, _ := strconv.Atoi(c[4])
			if got := Percent(covered, total); got != wantPct {
				t.Errorf("org.tsv:%d %s Percent(%d,%d)=%d want %d", r.line, c[1], covered, total, got, wantPct)
			}
			parts = append(parts, Counts{Covered: covered, Total: total})
		}
		want, _ := strconv.Atoi(m[1])
		if got := RunPercent(parts); got != want {
			t.Errorf("org.tsv:%d run %s: RunPercent=%d want %d", r.line, r.subject, got, want)
		}
		cliChecked++
	}
	if cliChecked < 40 {
		t.Fatalf("checked %d CLI runs", cliChecked)
	}
}

var deployCov = regexp.MustCompile(`coverage=(\S*) warnings=(.*)$`)
var deployStatus = regexp.MustCompile(`^status=(\w+) `)

// NATIVE deployment gates, API 67.0, one fresh org per scenario.
func TestDeployGateNative(t *testing.T) {
	rows := readNativeRows(t)
	n := 0
	for _, r := range rows {
		if r.kind != "deploy" {
			continue
		}
		n++
		status := deployStatus.FindStringSubmatch(r.detail)[1]
		m := deployCov.FindStringSubmatch(r.detail)
		var parts []Counts
		classes := map[string]Counts{}
		triggers := map[string]Counts{}
		for _, item := range strings.Split(m[1], ";") {
			if item == "" {
				continue
			}
			nameCounts := strings.SplitN(item, ":", 2)
			cc := strings.SplitN(nameCounts[1], "/", 2)
			covered, _ := strconv.Atoi(cc[0])
			total, _ := strconv.Atoi(cc[1])
			c := Counts{Covered: covered, Total: total}
			parts = append(parts, c)
			if strings.HasPrefix(nameCounts[0], "CvTrig") {
				triggers[nameCounts[0]] = c
			} else {
				classes[nameCounts[0]] = c
			}
		}
		var warnings []string
		pass := true
		if r.step == "RunSpecifiedTests" {
			for _, c := range classes {
				if ok, w := SpecifiedClassGate(c); !ok {
					pass = false
					warnings = append(warnings, w)
				}
			}
		} else {
			if _, ok, w := OverallGate(parts); !ok {
				pass = false
				warnings = append(warnings, w)
			}
			for _, c := range triggers {
				if ok, w := TriggerGate(c); !ok {
					pass = false
					warnings = append(warnings, w)
				}
			}
		}
		wantPass := status == "Succeeded"
		if pass != wantPass {
			t.Errorf("org.tsv:%d scenario %s (%s): gate pass=%v native status=%s", r.line, r.subject, r.step, pass, status)
		}
		if !pass && r.detail != "" {
			for _, w := range warnings {
				if !strings.Contains(m[2], w) {
					t.Errorf("org.tsv:%d scenario %s: warning %q not in native %q", r.line, r.subject, w, m[2])
				}
			}
		}
	}
	if n != 9 {
		t.Fatalf("deploy rows = %d", n)
	}
}

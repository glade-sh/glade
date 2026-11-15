package apextest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// TestTriggerAssertWrapConformance replays native rows for assertions (and,
// as controls, thrown exceptions and addError) raised inside Account triggers
// fired by DML. Native treats an AssertException like any trigger exception:
// the DML raises DmlException CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY whose text is
// "<Trigger>: execution of <Event>\n\ncaused by: <cause>\n\n<frames>".
// Rows: testdata/trigger_assert_wrap/org.tsv (APIs 62.0 and 67.0, named tests
// and anonymous Apex; identical at both versions).
// The captured script ran every cell as one block, so its stack texts end with
// the cell's own AnonymousBlock line; the line number is the script's, not the row's.
var anonymousBlockFrames = regexp.MustCompile(`(\nAnonymousBlock: line \d+, column 1)+`)

func TestTriggerAssertWrapConformance(t *testing.T) {
	dir := filepath.Join("testdata", "trigger_assert_wrap")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	type row struct{ context, name, api, value string }
	var rows []row
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 5)
		if len(fields) != 5 || fields[0] == "api" || fields[3] != "observed" {
			continue
		}
		var payload struct {
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(fields[4]), &payload); err != nil {
			t.Fatalf("row %s/%s: %v", fields[0], fields[2], err)
		}
		rows = append(rows, row{context: fields[1], name: fields[2], api: fields[0], value: payload.Value})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) < 300 {
		t.Fatalf("captured rows = %d", len(rows))
	}
	byName := map[string]map[string]string{}
	for _, r := range rows {
		key := r.context + "/" + r.name
		if byName[key] == nil {
			byName[key] = map[string]string{}
		}
		byName[key][r.api] = r.value
	}
	for key, versions := range byName {
		if versions["62.0"] != versions["67.0"] {
			t.Fatalf("captured rows for %s differ between 62.0 and 67.0", key)
		}
	}

	index := loadTestIndex(t, dir)
	if index.HasErrors() {
		t.Fatalf("synthetic project index: %v", index.Diagnostics)
	}
	if analysis := sema.Analyze(index); analysis.HasErrors() {
		t.Fatalf("diagnostics: %v", analysis.Diagnostics)
	}
	runners := map[string]*conformanceRunner{}
	for _, api := range []string{"62.0", "67.0"} {
		org := capturedFixtureOrg(t, index, api)
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		triggers, triggerErrors := compileProjectTriggers(index)
		if len(triggerErrors) > 0 {
			t.Fatal(triggerErrors)
		}
		for _, trigger := range triggers {
			if err := runner.base.RegisterTrigger(trigger); err != nil {
				t.Fatal(err)
			}
		}
		runners[api] = runner
	}
	run := Run(index, Options{SelectedClasses: []string{"TWTests"}, NoDiskCache: true})
	got := map[string]string{}
	gotStack := map[string][]string{}
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Problem == nil {
				t.Fatalf("%s passed; every cell reports through a failing test", testCase.MethodName)
			}
			message := testCase.Problem.Message
			if strings.HasPrefix(testCase.MethodName, "unc") {
				for _, frame := range testCase.Problem.Stack {
					gotStack[testCase.MethodName] = append(gotStack[testCase.MethodName], fmt.Sprintf("Class.%s: line %d, column 1", frame.Symbol, frame.Line))
				}
				got[testCase.MethodName] = testCase.Problem.Type + ": " + message
				continue
			}
			const marker = "Actual: "
			i := strings.Index(message, marker)
			if i < 0 {
				// The cell's own insert failed outside its catch.
				got[testCase.MethodName] = "UNCAUGHT " + testCase.Problem.Type + ": " + message
				continue
			}
			got[testCase.MethodName] = message[i+len(marker):]
		}
	}
	checked := 0
	for key, versions := range byName {
		context, name, _ := strings.Cut(key, "/")
		want := versions["67.0"]
		switch {
		case context == "isTest" && strings.HasPrefix(name, "unc"):
			// Native: "<type>: <message>|<stack>"; the stack is the test frame.
			message, nativeStack, _ := strings.Cut(want, "|Class.TWTests.")
			nativeFrames := []string{"Class.TWTests." + nativeStack}
			if frames := gotStack[name]; len(frames) != len(nativeFrames) || (len(frames) > 0 && frames[0] != nativeFrames[0]) {
				t.Errorf("test %s stack\n got: %q\nwant: %q", name, frames, nativeFrames)
			}
			message = strings.TrimPrefix(message, "System.")
			actual, ok := got[name]
			if !ok {
				t.Errorf("test %s did not run", name)
			} else if actual != message {
				t.Errorf("test %s\n got: %q\nwant: %q", name, actual, message)
			}
			checked++
		case context == "isTest":
			actual, ok := got[name]
			if !ok {
				t.Errorf("test %s did not run", name)
			} else if actual != want {
				t.Errorf("test %s\n got: %q\nwant: %q", name, actual, want)
			}
			checked++
		case strings.HasPrefix(name, "anon_"):
			base := strings.TrimPrefix(name, "anon_")
			cut := strings.LastIndex(base, "_")
			kind, key := base[cut+1:], strings.ReplaceAll(base[:cut], "_", "-")
			for _, api := range []string{"62.0", "67.0"} {
				program, err := vm.CompileAnonymousWithOptions("System.debug('OBS|'+TWHelper.cell('"+kind+"','"+key+"'));", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runners[api].execute(program)
				actual := ""
				if err != nil {
					actual = "UNCAUGHT " + err.Error()
				}
				for _, line := range result.Debug {
					if strings.HasPrefix(line, "OBS|") {
						actual = strings.TrimPrefix(line, "OBS|")
					}
				}
				// Replay uses a source-67 helper and a one-line anonymous
				// program. Matching-source rows retain native caller multiplicity
				// at the replay's actual line. The mixed-source API 62 replay
				// retains its earlier comparison scope.
				replayWant := anonymousBlockFrames.ReplaceAllString(want, "")
				if api == "67.0" {
					replayWant = anonymousBlockFrames.ReplaceAllStringFunc(want, func(frames string) string {
						return strings.Repeat("\nAnonymousBlock: line 1, column 1", strings.Count(frames, "AnonymousBlock: line "))
					})
				}
				if actual != replayWant {
					t.Errorf("anonymous %s at %s\n got: %q\nwant: %q", name, api, actual, replayWant)
				}
			}
			checked++
		case strings.HasPrefix(name, "anonunc_"):
			key := strings.ReplaceAll(strings.TrimPrefix(name, "anonunc_"), "_", "-")
			program, err := vm.CompileAnonymousWithOptions("insert new Account(Name='"+key+"');", vm.CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			_, execErr := runners["67.0"].execute(program)
			if execErr == nil {
				t.Errorf("anonymous %s did not fail", name)
			} else if !strings.Contains(execErr.Error(), strings.TrimSuffix(strings.TrimPrefix(want, "System.DmlException: "), ": []")) {
				t.Errorf("anonymous %s\n got: %q\nwant: %q", name, execErr.Error(), want)
			}
			checked++
		}
	}
	if checked != len(byName) {
		t.Fatalf("checked %d of %d rows", checked, len(byName))
	}
}

// TestTriggerAssertWrapTestResultStackAPI62 replays the isTest unc_* stacks at
// API 62.0 (the rows are identical at 62.0 and 67.0). The test-result stack
// holds one frame per Apex method, with the DML statement's line, like native.
func TestTriggerAssertWrapTestResultStackAPI62(t *testing.T) {
	src := filepath.Join("testdata", "trigger_assert_wrap")
	dir := t.TempDir()
	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dir, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, "-meta.xml") || strings.HasSuffix(path, "sfdx-project.json") {
			data = []byte(strings.ReplaceAll(string(data), "67.0", "62.0"))
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	native := map[string]string{}
	file, err := os.Open(filepath.Join(src, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 5)
		if len(fields) != 5 || fields[0] != "62.0" || fields[1] != "isTest" || fields[3] != "observed" || !strings.HasPrefix(fields[2], "unc") {
			continue
		}
		var payload struct {
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(fields[4]), &payload); err != nil {
			t.Fatal(err)
		}
		_, stack, _ := strings.Cut(payload.Value, "|Class.TWTests.")
		native[fields[2]] = "Class.TWTests." + stack
	}
	index := loadTestIndex(t, dir)
	if index.HasErrors() {
		t.Fatalf("synthetic project index: %v", index.Diagnostics)
	}
	run := Run(index, Options{SelectedClasses: []string{"TWTests"}, NoDiskCache: true})
	checked := 0
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			want, ok := native[testCase.MethodName]
			if !ok || testCase.Problem == nil {
				continue
			}
			var frames []string
			for _, frame := range testCase.Problem.Stack {
				frames = append(frames, fmt.Sprintf("Class.%s: line %d, column 1", frame.Symbol, frame.Line))
			}
			if len(frames) != 1 || frames[0] != want {
				t.Errorf("test %s stack at 62.0\n got: %q\nwant: %q", testCase.MethodName, frames, want)
			}
			checked++
		}
	}
	if checked != len(native) {
		t.Fatalf("checked %d of %d native stacks", checked, len(native))
	}
}

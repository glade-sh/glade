//go:build darwin || linux

package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/glade-sh/glade/internal/vm"
)

// Native ipv4_validation/literal_splits/org.tsv:2-5 (ORG-RUN.txt) pin
// escaped-dot String.split and Pattern.split results at API 62.0 and 67.0,
// anonymous and isTest. A resumed host can make the regex engine's
// approximate clock report a timeout before any search work ran. Every route
// below runs its first escaped-dot split straight after such a resume and must
// still produce the native cells: the anonymous executor and the project test
// runner, each with String.split first and Pattern.split first.
func TestRegexSplitResumedClockProjectAnonymousAndIsTestRoutes(t *testing.T) {
	const childFlag = "GLADE_REGEX_SPLIT_ROUTE_CLOCK_CHILD"
	if os.Getenv(childFlag) != "1" {
		ipv4ProjectSuspendedChild(t, "^TestRegexSplitResumedClockProjectAnonymousAndIsTestRoutes$", childFlag)
		return
	}
	native := literalSplitNativeCells(t)
	clock := regexp2.MustCompile(`\.`, 0)
	clock.MatchTimeout = 2 * time.Second
	resume := func() {
		// Keep the engine clock live, then suspend so it is stale on resume.
		if _, err := clock.FindStringMatch("0.0.0.0"); err != nil {
			t.Fatal(err)
		}
		fmt.Println("IPV4_PROJECT_PAUSE")
		if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
			t.Fatal(err)
		}
	}
	probes := map[string]string{
		"stringFirst":  "SplitResumeProbe.stringFirst()",
		"patternFirst": "SplitResumeProbe.patternFirst()",
		"observe":      "DelimiterBoundaryProbe.observe()",
	}
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "ipv4_validation", "literal_splits", "project"))); err != nil {
			t.Fatal(err)
		}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || (!strings.HasSuffix(path, ".cls-meta.xml") && entry.Name() != "sfdx-project.json") {
				return walkErr
			}
			text, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(path, []byte(strings.ReplaceAll(string(text), "67.0", api)), 0600)
		}); err != nil {
			t.Fatal(err)
		}
		classes := filepath.Join(root, "force-app/main/default/classes")
		writeFile(t, filepath.Join(classes, "SplitResumeProbe.cls"), `public class SplitResumeProbe {
 public static Map<String,Object> stringFirst() {
  List<String> first='0.0.0.0'.split('\\.');
  Map<String,Object> rows=new Map<String,Object>();
  rows.put('String/default/0',first);
  rows.put('String/0/2','0.0.0.0'.split('\\.',2));
  rows.put('Pattern/default/0',Pattern.compile('\\.').split('0.0.0.0'));
  rows.put('Pattern/0/2',Pattern.compile('\\.').split('0.0.0.0',2));
  return rows;
 }
 public static Map<String,Object> patternFirst() {
  List<String> first=Pattern.compile('\\.').split('0.0.0.0');
  Map<String,Object> rows=new Map<String,Object>();
  rows.put('Pattern/default/0',first);
  rows.put('Pattern/0/2',Pattern.compile('\\.').split('0.0.0.0',2));
  rows.put('String/default/0','0.0.0.0'.split('\\.'));
  rows.put('String/0/2','0.0.0.0'.split('\\.',2));
  return rows;
 }
}`)
		writeFile(t, filepath.Join(classes, "SplitResumeProbeTest.cls"), `@IsTest private class SplitResumeProbeTest {
 @IsTest static void stringFirst() { System.assertEquals('capture',JSON.serialize(SplitResumeProbe.stringFirst()),'DELIMITER_ROWS'); }
 @IsTest static void patternFirst() { System.assertEquals('capture',JSON.serialize(SplitResumeProbe.patternFirst()),'DELIMITER_ROWS'); }
}`)
		for _, name := range []string{"SplitResumeProbe", "SplitResumeProbeTest"} {
			writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
		}
		index := loadTestIndex(t, root)
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		for _, probe := range []string{"stringFirst", "patternFirst", "observe"} {
			label := api + "/anonymous/" + probe
			program, err := vm.CompileAnonymousWithOptions(`String observation=JSON.serialize(`+probes[probe]+`);`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := runner.newMachine()
			resume()
			if _, err := machine.Execute(program); err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			assertNativeSplitCells(t, label, machine.Globals["observation"].Text, native[api+"/anonymous"], probe == "observe")
		}
		result := Run(index, Options{
			SelectedClasses: []string{"SplitResumeProbeTest", "DelimiterBoundaryProbeTest"},
			NoDiskCache:     true,
			Parallelism:     1,
			Progress: func(event TestProgress) {
				if event.Event == "test_start" {
					resume()
				}
			},
		})
		seen := map[string]bool{}
		for _, suite := range result.Suites {
			for _, testCase := range suite.Cases {
				label := api + "/isTest/" + testCase.MethodName
				if testCase.Problem == nil || !strings.Contains(testCase.Problem.Message, "DELIMITER_ROWS") {
					t.Fatalf("%s: missing native observation transport: %+v", label, testCase.Problem)
				}
				_, observed, found := strings.Cut(testCase.Problem.Message, ", Actual: ")
				if !found {
					t.Fatalf("%s: unexpected observation transport: %s", label, testCase.Problem.Message)
				}
				assertNativeSplitCells(t, label, observed, native[api+"/isTest"], testCase.MethodName == "observe")
				seen[testCase.MethodName] = true
			}
		}
		for probe := range probes {
			if !seen[probe] {
				t.Fatalf("%s/isTest/%s did not run: %s", api, probe, firstRunProblem(result))
			}
		}
	}
}

func literalSplitNativeCells(t *testing.T) map[string]map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "ipv4_validation", "literal_splits", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	cells := map[string]map[string][]string{}
	for _, row := range rows[1:] {
		var values map[string][]string
		if len(row) != 4 || json.Unmarshal([]byte(row[3]), &values) != nil {
			t.Fatal("invalid native row")
		}
		cells[row[0]+"/"+row[1]] = values
	}
	for _, cell := range []string{"62.0/anonymous", "62.0/isTest", "67.0/anonymous", "67.0/isTest"} {
		if len(cells[cell]) != 96 {
			t.Fatalf("native cell %s has %d observations", cell, len(cells[cell]))
		}
	}
	return cells
}

func assertNativeSplitCells(t *testing.T, label, observed string, native map[string][]string, whole bool) {
	t.Helper()
	var got map[string][]string
	if err := json.Unmarshal([]byte(observed), &got); err != nil {
		t.Fatalf("%s: %v: %s", label, err, observed)
	}
	if whole && len(got) != len(native) {
		t.Fatalf("%s: observation inventory=%d want %d", label, len(got), len(native))
	}
	for key, value := range got {
		want, ok := native[key]
		if !ok || !reflect.DeepEqual(value, want) {
			t.Fatalf("%s: %s=%q want native %q", label, key, value, want)
		}
	}
}

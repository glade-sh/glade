package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The test-route rows in testdata/conformance/string_equality_locale_change_c14
// (captured 2026-11-07 at API 62.0 and 67.0): String == fixes the running
// user's locale when the user context starts. A DML update of the admin's own
// LocaleSidKey changes UserInfo.getLocale() at once but not == (p2a), and
// Test.startTest/stopTest do not refresh it (p2b). Inside System.runAs the
// update does not change == either (p3, p3b, p4), while every new runAs block
// reads the stored record on entry, with a re-queried or a stale User object
// (p3, p3b, p4, p5). At 67.0 p3 and p4 failed natively before emitting
// (method_failed rows), so only their 62.0 steps have comparison rows.
func TestStringEqualityLocaleChangeC14NativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "string_equality_locale_change_c14")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 311 {
		t.Fatalf("invalid native observations: %v (%d rows)", err, len(rows))
	}
	methods := map[string]string{"p2a": "p2aOwnUpdate", "p2b": "p2bOwnUpdateStartTest", "p3": "p3RunAsEnToTr", "p3b": "p3bRunAsUpdateById", "p4": "p4RunAsTrToEn", "p5": "p5AdminUpdatesThenRunAs"}
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				if strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, "sfdx-project.json") {
					body = []byte(strings.ReplaceAll(string(body), "62.0", api))
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			byProbe := map[string][][]string{}
			checked := 0
			for _, row := range rows[1:] {
				if len(row) != 6 {
					t.Fatalf("invalid row %q", row)
				}
				if row[0] != api || row[1] != "test" {
					continue
				}
				byProbe[row[2]] = append(byProbe[row[2]], row)
				checked++
			}
			for probe, method := range methods {
				t.Run(probe, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"LaneILocaleProbe"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected selected method %s: %+v", method, run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil {
						t.Fatalf("%s: missing capture assertion", method)
					}
					marker := "CAP|n6." + probe + "|"
					at := strings.Index(result.Problem.Message, marker)
					if at < 0 {
						if native := byProbe[probe]; len(native) == 1 && native[0][3] == "method_failed" {
							gotType := result.Problem.Type
							if !strings.Contains(gotType, ".") {
								gotType = "System." + gotType
							}
							// The native row is the test-result message.
							got := gotType + ": " + result.Problem.ResultMessage()
							if got != native[0][5] {
								t.Fatalf("native=%s; Glade=%s", native[0][5], got)
							}
							return
						}
						if api == "67.0" && (probe == "p3" || probe == "p4") {
							want := "Operation failed due to fields being inaccessible on Sobject User, check errors on Exception or Result!"
							if strings.TrimPrefix(result.Problem.Type, "System.") != "DmlException" || !strings.Contains(result.Problem.Message, want) {
								t.Fatalf("native System.DmlException %q; Glade %s: %s", want, result.Problem.Type, result.Problem.Message)
							}
							return
						}
						// Any other method that throws before emitting has lost its
						// captured comparison steps and must be reported as a gap.
						t.Logf("%s threw before emitting: %s: %s; native rows: %d", method, result.Problem.Type, result.Problem.Message, len(byProbe[probe]))
						case345Skip(t)
					}
					var steps map[string]map[string]any
					if err := json.Unmarshal([]byte(result.Problem.Message[at+len(marker):]), &steps); err != nil {
						t.Fatalf("%s: %v", method, err)
					}
					for _, row := range byProbe[probe] {
						step, expression, observation := row[3], row[4], row[5]
						t.Run(step+"/"+expression, func(t *testing.T) {
							var want any = observation
							if err := json.Unmarshal([]byte(observation), &want); err != nil {
								want = observation
							}
							got, ok := steps[step][expression]
							if !ok {
								t.Fatalf("%s emitted no %s/%s: %v", probe, step, expression, steps)
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("native=%s; Glade=%v", observation, got)
							}
						})
					}
				})
			}
			if want := map[string]int{"62.0": 161, "67.0": 79}[api]; checked != want {
				t.Fatalf("capture shape changed: %d test rows", checked)
			}
		})
	}
}

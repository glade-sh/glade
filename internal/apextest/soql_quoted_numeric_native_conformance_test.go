package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// org.tsv captures the same owned literals through anonymous and named source
// at both supported API boundaries. Named runtime replay also covers warm caches.
func TestSOQLQuotedNumericNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/soql_quoted_numeric"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 289 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	data, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
				t.Fatalf("native project compiled: %v %v", index.Diagnostics, a.Diagnostics)
			}
			compare := func(t *testing.T, message, want string) {
				t.Helper()
				_, payload, ok := strings.Cut(message, "OBS|")
				if !ok {
					t.Fatalf("missing observation: %s", message)
				}
				var gotValue, wantValue any
				if json.Unmarshal([]byte(payload), &gotValue) != nil || json.Unmarshal([]byte(want), &wantValue) != nil {
					t.Fatalf("invalid JSON: %s / %s", payload, want)
				}
				if !reflect.DeepEqual(gotValue, wantValue) {
					t.Errorf("native=%s; Glade=%s", want, payload)
				}
			}
			for _, row := range rows[1:] {
				if len(row) != 5 {
					t.Fatalf("invalid native row: %v", row)
				}
				if row[0] != api || row[1] != "anonymous" || strings.HasPrefix(row[3], "static_") {
					continue
				}
				t.Run("anonymous/"+row[2]+"/"+row[3], func(t *testing.T) {
					source := "System.assert(false,'OBS|'+JSON.serialize(QuotedNumericProbe.observe('" + row[2] + "','" + row[3] + "')));"
					if a := sema.AnalyzeAnonymous(index, source, api); a.HasErrors() {
						t.Fatal(a.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					_, err = runner.execute(program)
					var runtimeErr *vm.RuntimeError
					if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
						t.Fatalf("observation: %v", err)
					}
					compare(t, runtimeErr.ExceptionMessage(), row[4])
				})
			}
			t.Setenv("GLADE_HOME", t.TempDir())
			for _, phase := range []string{"cold", "warm"} {
				t.Run(phase, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"QuotedNumericProbeTest"}, Parallelism: 1})
					problems := map[string]string{}
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								problems[c.MethodName] = c.Problem.Message
							}
						}
					}
					if len(problems) != 40 {
						t.Fatalf("named observations=%d: %+v", len(problems), run.Summary())
					}
					for _, row := range rows[1:] {
						if row[0] != api || row[1] != "isTest" || strings.HasPrefix(row[3], "static_") {
							continue
						}
						t.Run(row[2]+"/"+row[3], func(t *testing.T) { compare(t, problems[row[2]+"_"+row[3]], row[4]) })
					}
				})
			}
			for _, row := range rows[1:] {
				if row[0] != api || !strings.HasPrefix(row[3], "static_") {
					continue
				}
				t.Run(row[1]+"/"+row[2]+"/"+row[3], func(t *testing.T) {
					predicate, ok := cases[row[2]]
					if !ok {
						t.Fatal("missing native source")
					}
					literal := "SELECT Id FROM Account WHERE " + predicate
					target := "List<SObject>"
					if row[3] == "static_sosl" {
						literal = "FIND 'quotednumericnomatch' IN ALL FIELDS RETURNING Account(Id WHERE " + predicate + ")"
						target = "List<List<SObject>>"
					}
					source := target + " value = [" + literal + "];"
					var want struct {
						Compiled bool   `json:"compiled"`
						Message  string `json:"message"`
					}
					if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
						t.Fatal(err)
					}
					check := func(diagnostics []diagnostic.Diagnostic) {
						t.Helper()
						rejected := false
						nativeMessage := want.Message[strings.LastIndex(want.Message, "\n")+1:]
						matched := false
						for _, d := range diagnostics {
							if d.Severity == diagnostic.Error {
								rejected = true
							}
							if nativeMessage != "" && (strings.Contains(d.NativeMessage, nativeMessage) || strings.Contains(d.Message, nativeMessage)) {
								matched = true
							}
						}
						if rejected == want.Compiled {
							t.Errorf("native compiled=%v; diagnostics=%v", want.Compiled, diagnostics)
						}
						if !want.Compiled && !matched {
							t.Errorf("missing native diagnostic %q: %v", nativeMessage, diagnostics)
						}
					}
					if row[1] == "anonymous" {
						for _, phase := range []string{"cold", "warm"} {
							t.Run(phase, func(t *testing.T) { check(sema.AnalyzeAnonymous(index, source, api).Diagnostics) })
						}
					} else {
						path := filepath.Join(root, "force-app/main/default/classes/QuotedStaticProbeTest")
						writeFile(t, path+".cls", "@IsTest public class QuotedStaticProbeTest { @IsTest static void probe() { "+source+" } }")
						writeFile(t, path+".cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
						namedIndex := loadTestIndex(t, root)
						analysis := sema.Analyze(namedIndex)
						check(append(namedIndex.Diagnostics, analysis.Diagnostics...))
					}
				})
			}
		})
	}
}

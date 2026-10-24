package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestIncrementValueRoutesNativeConformance(t *testing.T) {
	runIncrementValueRoutesNativeConformance(t, false)
}

// Comments preserve the captured rejection token stream, including semicolons
// embedded in comments before and after the postfix operator.
func TestIncrementValueRoutesPostfixCommentsConformance(t *testing.T) {
	runIncrementValueRoutesNativeConformance(t, true)
}

func runIncrementValueRoutesNativeConformance(t *testing.T, comments bool) {
	const dir = "testdata/conformance/increment_value_routes"
	transcript, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cases.json", "OperatorCell.cls", "OperatorObservationTest.cls", "consumed_list_diagnostic.apex"} {
		source, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(transcript), fmt.Sprintf("sha256 %s %x", name, sha256.Sum256(source))) {
			t.Fatalf("captured source changed: %s", name)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Body, Kind string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	for _, row := range cases {
		bodies[row.ID] = row.Body
	}
	helper, err := os.ReadFile(filepath.Join(dir, "OperatorCell.cls"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 149 {
		t.Fatalf("native rows: %d", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if comments && row[2] != "string_local" && row[2] != "string_field" && row[2] != "boxed_object" {
			continue
		}
		key := strings.Join(row[:3], "/")
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") || (row[3] != "runtime" && row[3] != "rejection") {
			t.Fatalf("invalid native cell: %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			body := bodies[row[2]]
			if comments {
				body = strings.Replace(body, "++;", "/* ; */++/* ; */;", 1)
			}
			if body == "" {
				t.Fatal("missing captured body")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[0]+`"}`)
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			writeFile(t, filepath.Join(classes, "OperatorCell.cls"), string(helper))
			writeFile(t, filepath.Join(classes, "OperatorCell.cls-meta.xml"), "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			if row[1] == "isTest" {
				terminal := ""
				if row[3] == "runtime" {
					terminal = "\nSystem.assert(false,'OBS|'+JSON.serialize(observations));"
				}
				writeFile(t, filepath.Join(classes, "OperatorObservationTest.cls"), "@IsTest private class OperatorObservationTest { @IsTest static void "+row[2]+"() {\n"+body+terminal+"\n} }")
				writeFile(t, filepath.Join(classes, "OperatorObservationTest.cls-meta.xml"), "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if row[3] == "rejection" {
				if row[1] == "anonymous" {
					if result := sema.AnalyzeAnonymous(index, body, row[0]); !result.HasErrors() {
						t.Fatal("native rejected; anonymous project gate accepted")
					}
				} else {
					run := Run(index, Options{SelectedClasses: []string{"OperatorObservationTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.CompileErrors != 1 {
						t.Fatalf("native rejected; project test runner: %+v", summary)
					}
				}
				return
			}
			if result := sema.Analyze(index); result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
			observed := ""
			if row[1] == "anonymous" {
				source := body + "\nSystem.debug('OBS|'+JSON.serialize(observations));"
				if result := sema.AnalyzeAnonymous(index, source, row[0]); result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0], ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, source, row[0])})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = row[0]
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, value, ok := strings.Cut(line, "OBS|"); ok {
						observed = value
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{"OperatorObservationTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("test runner: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("missing native assertion transport: %+v", result.Problem)
				}
				_, observed, _ = strings.Cut(result.Problem.Message, "OBS|")
			}
			var got, want []string
			if err := json.Unmarshal([]byte(observed), &got); err != nil {
				t.Fatalf("observation %q: %v", observed, err)
			}
			if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}

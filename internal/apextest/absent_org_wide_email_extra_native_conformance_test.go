package apextest

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestAbsentOrgWideEmailExtraNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/absent_org_wide_email"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		id, api, route := row[0], row[1], row[2]
		if id != "template" && id != "queue" {
			continue
		}
		key := id + "/" + api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			classNames := []string{"Road548Template"}
			if id == "queue" {
				classNames = []string{"Road548Queue", "Road548QueueTest"}
			}
			for _, name := range classNames {
				rel := filepath.Join("force-app/main/default/classes", name+".cls")
				source, err := os.ReadFile(filepath.Join(fixture, rel))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, rel), string(source))
				writeFile(t, filepath.Join(root, rel+"-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			seedOrgWideEmailCatalog(&org)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			actual := []string{id, api, route, "true", "", "", "n/a", "n/a", "n/a"}
			if strings.HasSuffix(route, "anonymous") {
				source, err := os.ReadFile(filepath.Join(fixture, id+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := runner.newMachine()
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				prefix := "ROAD548_TEMPLATE="
				if id == "queue" {
					actual[6] = api
					if err := machine.DrainAsync(&result); err != nil {
						t.Fatal(err)
					}
					query, err := vm.CompileAnonymousWithOptions(`System.debug('ROAD548=' + [SELECT Description FROM Account WHERE Name='Road548Queue670'].Description);`, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err = machine.Execute(query)
					if err != nil {
						t.Fatal(err)
					}
					prefix = "ROAD548="
				}
				actual[4] = "true"
				if len(result.Debug) != 1 {
					t.Fatalf("debug = %q", result.Debug)
				}
				var ok bool
				actual[5], ok = strings.CutPrefix(result.Debug[0], prefix)
				if !ok {
					t.Fatalf("debug = %q", result.Debug)
				}
			} else {
				className, method := "Road548Template", "testTemplate"
				if id == "queue" {
					className, method = "Road548QueueTest", "runQueue"
				}
				cases := Discover(index, Options{SelectedClasses: []string{className}})
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				runtimeMethods := indexTestRuntimeMethods(methods)
				count := 0
				for _, tc := range cases {
					if !strings.EqualFold(tc.MethodName, method) {
						continue
					}
					count++
					k := testCaseKey(tc)
					result := runCase(context.Background(), tc, methods[k], runtimeMethods[testMethodSourceKey(tc.ClassName, tc.File)], methodErrors[k], programs[k], programErrors[k], runner.base, nil, nil, nil, org, 0, Options{NoDiskCache: true, Parallelism: 1}, true, nil, newRunPerfCounters(false))
					actual[4] = string(result.Status)
					if actual[4] == "fail" {
						actual[4] = "Failed"
					}
					if result.Problem == nil || result.Problem.Type != "System.AssertException" {
						t.Fatalf("unexpected result: %+v", result)
					}
					var ok bool
					actual[5], ok = strings.CutPrefix(result.Problem.Message, "Assertion Failed: ROAD548=")
					if !ok {
						t.Fatalf("unexpected assertion: %s", result.Problem.Message)
					}
				}
				if count != 1 {
					t.Fatalf("found %d tests", count)
				}
				actual[6] = api
			}
			if !reflect.DeepEqual(actual, row) {
				t.Fatalf("got %q, want %q", actual, row)
			}
		})
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, pair := range [][2]string{{"template", "anonymous"}, {"template", "istest"}, {"queue", "queue_anonymous"}, {"queue", "queue_istest"}} {
			key := pair[0] + "/" + api + "/" + pair[1]
			if !seen[key] {
				t.Fatalf("missing row %s", key)
			}
		}
	}
}

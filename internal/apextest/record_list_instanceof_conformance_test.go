package apextest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// instanceof against SObject, List<SObject>, List<Account> and Object for
// record lists, a record and null, directly and through an Object parameter,
// plus the explicit (SObject) cast. Every API and route has its own native
// answer; the sources are shared.
func TestRecordListInstanceOfOrgConformance(t *testing.T) {
	var data struct {
		APIVersions  []string `json:"apiVersions"`
		Routes       []string `json:"routes"`
		MarkerPrefix string   `json:"markerPrefix"`
		RowCount     int      `json:"rowCount"`
		Rows         []struct {
			ID        string            `json:"id"`
			Helpers   map[string]string `json:"helpers"`
			Anonymous string            `json:"anonymous"`
			Classes   map[string]string `json:"classes"`
			Tests     []string          `json:"tests"`
			Routes    map[string]struct {
				ByAPI map[string]stubDispatchExpectation `json:"byAPI"`
			} `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/record_list_instanceof.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" ||
		data.RowCount != 53 || len(data.Rows) != data.RowCount || data.MarkerPrefix == "" {
		t.Fatal("incomplete native instanceof matrix")
	}
	seen := make(map[string]bool)
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Tests) != 1 || len(row.Classes) != 1 || row.Anonymous == "" {
			t.Fatal("missing or repeated native instanceof row")
		}
		seen[row.ID] = true
		for _, route := range data.Routes {
			for _, api := range data.APIVersions {
				cell, ok := row.Routes[route].ByAPI[api]
				if !ok || (cell.Outcome == "runtime") == (cell.Primary != "") {
					t.Fatal("missing native instanceof answer")
				}
			}
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							cell := row.Routes[route].ByAPI[api]
							var classes map[string]string
							if route == "isTest" {
								classes = row.Classes
							}
							index := stubDispatchIndex(t, api, row.Helpers, classes)
							diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), sema.Analyze(index).Diagnostics...)
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
							var execErr error
							if route == "anonymous" {
								diagnostics = append(diagnostics, sema.AnalyzeAnonymous(index, row.Anonymous, api).Diagnostics...)
								program, compileErr := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: api})
								stubDispatchCompile(t, cell, diagnostics, compileErr)
								if cell.Outcome == "compile-rejected" {
									return
								}
								_, execErr = runner.execute(program)
							} else {
								stubDispatchCompile(t, cell, diagnostics, nil)
								if cell.Outcome == "compile-rejected" {
									return
								}
								cases := Discover(index, Options{SelectedClasses: row.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatal("native named entry not discovered")
								}
								methods, methodErrors := compileTestMethods(cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named entry compile: %v %v", methodErrors[key], programErrors[key])
								}
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							stubDispatchCompare(t, data.MarkerPrefix+row.ID+"|", cell, execErr)
						})
					}
				})
			}
		})
	}
}

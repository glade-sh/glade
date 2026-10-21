package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned native comparator rows cover API 61/62/67/68 and helper61 with newer
// callers, independently in anonymous Apex and @IsTest. Retain captured cast,
// invocation, instanceof and overload outcomes as distinct observations.
// Boolean/Object-parameter rows at 62/67/68 retain their owned native envelope.
func TestComparatorInitializerFloorConformance(t *testing.T) {
	var data struct {
		Rows []struct {
			ID        string            `json:"id"`
			Route     string            `json:"route"`
			HelperAPI string            `json:"helperApi"`
			CallerAPI string            `json:"callerApi"`
			Classes   map[string]string `json:"classes"`
			ClassAPIs map[string]string `json:"classApis"`
			Anonymous string            `json:"anonymous"`
			Tests     []string          `json:"tests"`
			Expected  json.RawMessage   `json:"expected"`
			Marker    string            `json:"marker"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/comparator_initializer_floor.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 202 {
		t.Fatal("incomplete native initializer matrix")
	}
	for _, row := range data.Rows {
		t.Run(row.ID+"/"+row.HelperAPI+"/"+row.CallerAPI+"/"+row.Route, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.CallerAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for name, source := range row.Classes {
				api := row.ClassAPIs[name]
				if api == "" {
					t.Fatal("missing class metadata version")
				}
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native fixture accepted; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			var execErr error
			switch row.Route {
			case "anonymous":
				checked := sema.AnalyzeAnonymous(index, row.Anonymous, row.CallerAPI)
				if checked.HasErrors() {
					t.Fatal(checked.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: row.CallerAPI})
				if err != nil {
					t.Fatal(err)
				}
				_, execErr = runner.execute(program)
			case "istest":
				cases := Discover(index, Options{SelectedClasses: row.Tests, SelectedMethod: "observe"})
				if len(row.Tests) != 1 || len(cases) != 1 {
					t.Fatal("named observation discovery differs")
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
			default:
				t.Fatal("unknown native route")
			}
			comparatorContractValue(t, row.Marker, genericInterfaceCastExpected{Outcome: "runtime", Value: string(row.Expected)}, execErr)
		})
	}
}

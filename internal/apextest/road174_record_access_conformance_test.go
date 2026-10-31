package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRoad174RecordAccessNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/conformance/road174_record_access/org.tsv")
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
	for _, row := range rows[1:] {
		row := row
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			api, route, object, outcome := row[0], row[1], row[2], row[3]
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			storage.EnsureStandardObject(&org, object)
			base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			query := "SELECT UserRecordAccess.HasReadAccess FROM " + object + " LIMIT 1"
			source := "Boolean caught=false;try{Database.query('" + query + "');}catch(QueryException e){caught=true;}System.assertEquals(" + fmt.Sprint(outcome == "reject") + ",caught);"
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := base.newMachine()
			machine.EnableTestContext()
			if route == "istest" {
				if err := machine.RegisterClass(vm.Class{Name: "Road174RecordAccess", APIVersion: api, Modifiers: []string{"isTest"}}); err != nil {
					t.Fatal(err)
				}
			}
			for pass := 0; pass < 2; pass++ {
				if route == "istest" {
					_, err = machine.ExecuteInClass(program, "Road174RecordAccess")
				} else {
					_, err = machine.Execute(program)
				}
				if err != nil {
					t.Fatalf("pass %d, native %s: %v", pass, outcome, err)
				}
			}
			if outcome == "ok" {
				analysis := sema.AnalyzeAnonymous(index, "List<"+object+"> result=["+query+"];", api)
				if analysis.HasErrors() {
					t.Fatalf("native accepts static query: %v", analysis.Diagnostics)
				}
			}
		})
	}
}

// The same native test classes run through project discovery and compilation.
func TestRoad174RecordAccessProject(t *testing.T) {
	root := "testdata/conformance/road174_record_access/project"
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	for _, name := range []string{"Lead", "Campaign", "Contract", "Asset", "Product2", "Pricebook2", "Order", "OpportunityLineItem", "Account", "Contact", "Case", "Task", "Event", "User", "Profile"} {
		storage.EnsureStandardObject(&org, name)
	}
	base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
	for _, class := range []string{"Road174Probe62", "Road174Probe67"} {
		t.Run(class, func(t *testing.T) {
			cases := Discover(index, Options{SelectedClasses: []string{class}, SelectedMethod: "check"})
			if len(cases) != 1 {
				t.Fatalf("discovered %d tests", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
			}
			machine := base.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
				t.Fatal(err)
			}
		})
	}
}

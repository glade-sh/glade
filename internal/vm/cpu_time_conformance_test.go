package vm_test

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCPUTimeNativeConformance(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := filepath.Join("testdata", "conformance", "cpu_time")
	p, err := project.Load(filepath.Join(root, "project"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "native", "api"+api[:2], "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			r := csv.NewReader(strings.NewReader(string(data)))
			r.Comma = '\t'
			rows, err := r.ReadAll()
			if err != nil || len(rows) != 7 {
				t.Fatalf("native rows: %v, %v", rows, err)
			}
			number := func(s string) int {
				n, err := strconv.Atoi(s)
				if err != nil {
					t.Fatal(err)
				}
				return n
			}
			if number(rows[1][3]) < 0 || number(rows[2][4]) <= number(rows[2][3]) || number(rows[2][5]) != 10000 || number(rows[3][3]) < 2000 || number(rows[3][4])*10 >= number(rows[3][3]) || rows[4][2] != "PASS" || rows[5][4] != "60000" || rows[6][2] != "System.LimitException" {
				t.Fatalf("native CPU contract changed: %v", rows)
			}
			source := `Integer before = Limits.getCpuTime(); Integer sum = 0; for(Integer i=0;i<1000;i++){sum += i;} System.assert(Limits.getCpuTime() >= before); System.assertEquals(10000, Limits.getLimitCpuTime());`
			t.Run("anonymous", func(t *testing.T) {
				machine := vm.New(nil)
				machine.SetLimitMode(vm.LimitModeStrict)
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
			class := "CpuProbe" + api[:2] + "Test"
			t.Run("IsTest", func(t *testing.T) {
				machine := vm.New(nil)
				machine.SetLimitMode(vm.LimitModeStrict)
				if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
					t.Fatal(err)
				}
				org := apextest.OrgFromIndex(index)
				org.APIVersion = api
				machine.SetOrg(&org)
				machine.EnableTestContext()
				if err := apextest.RegisterTestMethodForRequest(machine, index, class, "observe"); err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(class+".observe();", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteTestInClass(program, class); err != nil {
					t.Fatal(err)
				}
			})
			for run := 0; run < 2; run++ {
				t.Run(fmt.Sprintf("project-warm-%d", run), func(t *testing.T) {
					report := apextest.Run(index, apextest.Options{SelectedClasses: []string{class}, SelectedMethod: "observe", Parallelism: 1, LimitMode: vm.LimitModeStrict})
					summary := report.Summary()
					if summary.Total != 1 || summary.Passed != 1 || summary.Failed != 0 || summary.Errors != 0 {
						t.Fatalf("project CPU conformance: %+v, %+v", summary, report)
					}
				})
			}
			t.Run("exhaustion", func(t *testing.T) {
				machine := vm.New(nil)
				machine.SetLimitMode(vm.LimitModeStrict)
				caps, _ := vm.LimitCapsForProfile("strict-sync")
				caps.CPUTimeMS = 2 // Exercise the captured rejection with a short local cap.
				machine.SetLimitCaps(caps)
				if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions("CpuProbe"+api[:2]+".exhaust();", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				var limitErr *vm.RuntimeError
				if !errors.As(err, &limitErr) || limitErr.Type != rows[6][2] || len(result.Debug) != 0 {
					t.Fatalf("captured exhaustion must escape the Apex catch: %v, debug=%v", err, result.Debug)
				}
				if result.Limits.CPUTimeMS <= caps.CPUTimeMS || len(result.LimitViolations) != 1 || result.LimitViolations[0].Used != result.Limits.CPUTimeMS {
					t.Fatalf("exhaustion units: limits=%+v, violations=%v", result.Limits, result.LimitViolations)
				}
			})
		})
	}
}

func TestCPUTimeStrictBudgetTestRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"sfdx-project.json": fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api),
				"force-app/main/default/classes/CpuBudgetTest.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`,
				"force-app/main/default/classes/CpuBudgetTest.cls": `@IsTest private class CpuBudgetTest {
 @IsTest static void budget() {
  Integer until = Limits.getCpuTime() + 20;
  while (Limits.getCpuTime() < until) {}
  Test.startTest();
  System.assert(Limits.getCpuTime() < until);
  Integer childUntil = Limits.getCpuTime() + 20;
  while (Limits.getCpuTime() < childUntil) {}
  Test.stopTest();
  System.assert(Limits.getCpuTime() >= until);
  System.assert(Limits.getCpuTime() < Limits.getLimitCpuTime());
 }
}`,
			}
			for name, body := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Run("IsTest", func(t *testing.T) {
				machine := vm.New(nil)
				machine.SetLimitMode(vm.LimitModeStrict)
				if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
					t.Fatal(err)
				}
				org := apextest.OrgFromIndex(index)
				machine.SetOrg(&org)
				machine.EnableTestContext()
				if err := apextest.RegisterTestMethodForRequest(machine, index, "CpuBudgetTest", "budget"); err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions("CpuBudgetTest.budget();", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteTestInClass(program, "CpuBudgetTest"); err != nil {
					t.Fatal(err)
				}
			})
			for run := 0; run < 2; run++ {
				t.Run(fmt.Sprintf("project-warm-%d", run), func(t *testing.T) {
					report := apextest.Run(index, apextest.Options{SelectedClasses: []string{"CpuBudgetTest"}, SelectedMethod: "budget", Parallelism: 1, LimitMode: vm.LimitModeStrict})
					if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("strict budget test: %+v", report)
					}
				})
			}
		})
	}
}

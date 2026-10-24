package apextest

import (
	"context"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestPlatformCalloutEndpointNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			cases := readPlatformTransactionCases(t)
			anonymous := readPlatformTransactionNative(t, "callout_work", api, "anonymous", cases)
			named := readPlatformTransactionNative(t, "callout_work", api, "istest", cases)
			index := loadTestIndex(t, copyPlatformTransactionProject(t, "callout_work", api))
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var own []TestCase
			for _, row := range Discover(index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}}) {
				if row.MethodName != "W01" && row.MethodName != "W02" {
					continue
				}
				own = append(own, row)
				t.Run(row.MethodName+"/anonymous", func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions("CapturePlatformProbe.emit('"+row.MethodName+"');", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = runner.newMachine().Execute(program)
					assertPlatformTransactionObservation(t, row.MethodName, err, anonymous[row.MethodName])
				})
				t.Run(row.MethodName+"/named_istest", func(t *testing.T) {
					machine := runner.newMachine()
					machine.EnableTestContext()
					if err := RegisterTestMethodForRequest(machine, index, row.ClassName, row.MethodName); err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions(row.ClassName+"."+row.MethodName+"();", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.ExecuteTestInClass(program, row.ClassName)
					assertPlatformTransactionObservation(t, row.MethodName, err, named[row.MethodName])
				})
			}
			if len(own) != 2 {
				t.Fatalf("discovered %d callout methods, want 2", len(own))
			}
			t.Run("project_test_runner", func(t *testing.T) {
				report := RunCasesContext(context.Background(), index, Options{NoDiskCache: true, Parallelism: 1}, own)
				if report.Summary().Total != len(own) {
					t.Fatalf("missing project cases: %+v", report.Summary())
				}
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						t.Run(row.MethodName, func(t *testing.T) {
							if row.Problem == nil {
								t.Fatal("missing native observation")
							}
							assertPlatformTransactionPayload(t, row.MethodName, row.Problem.Message, named[row.MethodName])
						})
					}
				}
			})
		})
	}
}

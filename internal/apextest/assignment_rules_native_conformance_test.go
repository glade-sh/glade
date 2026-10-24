package apextest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestAssignmentRulesNativeRowsAPIRoutes(t *testing.T) {
	cases := readPlatformTransactionCases(t)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				rows := readPlatformTransactionNative(t, "assignment_rules", api, route, cases)
				if len(rows) != 5 {
					t.Fatalf("assignment native rows=%d, want 5", len(rows))
				}
			})
		}
	}
}

func TestAssignmentRulesNativeConformanceAPIRoutes(t *testing.T) {
	cases := readPlatformTransactionCases(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			anonymous := readPlatformTransactionNative(t, "assignment_rules", api, "anonymous", cases)
			named := readPlatformTransactionNative(t, "assignment_rules", api, "istest", cases)
			root := copyPlatformTransactionProject(t, "assignment_rules", api)
			index := loadTestIndex(t, root)
			t.Run("project_semantics", func(t *testing.T) {
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native project accepted; diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
			})
			var own []TestCase
			for _, row := range Discover(index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}}) {
				if named[row.MethodName] != nil {
					own = append(own, row)
				}
			}
			if len(own) != len(named) {
				t.Fatalf("discovered assignment tests=%d, want %d", len(own), len(named))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			methods, methodErrors := compileTestMethodsForIndex(&index, own)
			programs, programErrors := compileTestInvokePrograms(own)
			template, err := os.ReadFile(filepath.Join(platformTransactionFixture, "anonymous.apex"))
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range own {
				t.Run(row.MethodName, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(strings.ReplaceAll(string(template), "CASE_ID", row.MethodName), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						// The native anonymous capture runs as the scratch administrator.
						machine := runner.newMachine()
						machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
						_, err = machine.Execute(program)
						assertPlatformTransactionObservation(t, row.MethodName, err, anonymous[row.MethodName])
					})
					t.Run("named_istest", func(t *testing.T) {
						key := testCaseKey(row)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
						assertPlatformTransactionObservation(t, row.MethodName, err, named[row.MethodName])
					})
				})
			}
			t.Run("project_test_runner", func(t *testing.T) {
				report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}, NoDiskCache: true, Parallelism: 1}, own)
				results := map[string]testreport.Case{}
				for _, suite := range report.Suites {
					for _, result := range suite.Cases {
						results[result.MethodName] = result
					}
				}
				for _, row := range own {
					t.Run(row.MethodName, func(t *testing.T) {
						result, ok := results[row.MethodName]
						if !ok || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatalf("native observation completed; project status=%s; problem=%+v; exclusions=%v", result.Status, result.Problem, report.ExcludedClasses)
						}
						assertPlatformTransactionPayload(t, row.MethodName, result.Problem.Message, named[row.MethodName])
					})
				}
			})
		})
	}
}

func TestAssignmentRuleMetadataSnapshotAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyPlatformTransactionProject(t, "assignment_rules", api)
			index := loadTestIndex(t, root)
			sources := newSourceCache()
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "force-app", "main", "default", "assignmentRules", "Lead.assignmentRules-meta.xml")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(original)+"\n")
			captured, err := sources.readCapturedProjectMetadata(path)
			if err != nil || !bytes.Equal(captured, original) {
				t.Errorf("assignment snapshot lost native-deployed metadata after disk edit: %v", err)
			}
			if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
				t.Error("assignment metadata edit did not invalidate the captured generation")
			}
		})
	}
}

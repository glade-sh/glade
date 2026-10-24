package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestDescribeReadWorkloadProjectRoutes(t *testing.T) {
	// Reuse the captured read oracle; allocation counts are local guards.
	const fixture = "../vm/testdata/describe_read_workload"
	source, err := os.ReadFile(filepath.Join(fixture, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	file.Close()
	if err != nil || len(rows) != 19 || !reflect.DeepEqual(rows[0], []string{"source_api", "row", "observation"}) {
		t.Fatalf("incomplete native read matrix: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "DescribeReadBinding.cls"), `public class DescribeReadBinding {
    public static void observe() {`+string(source)+`}
    public static void bindGlobals() {
        Schema.SObjectType last;
        for (Integer i = 0; i < 16; i++) {
            Map<String,Schema.SObjectType> globals = Schema.getGlobalDescribe();
            last = globals.get('PARCEL000__C');
        }
        System.assert(last != null);
    }
}`)
			writeFile(t, filepath.Join(classes, "DescribeReadBindingTest.cls"), `@IsTest private class DescribeReadBindingTest {
    @IsTest static void observe() { DescribeReadBinding.observe(); }
    @IsTest static void bindGlobals() { DescribeReadBinding.bindGlobals(); }
}`)
			for _, class := range []string{"DescribeReadBinding", "DescribeReadBindingTest"} {
				writeFile(t, filepath.Join(classes, class+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			testCases := Discover(index, Options{SelectedClasses: []string{"DescribeReadBindingTest"}})
			if len(testCases) != 2 {
				t.Fatalf("named test methods: %d, want 2", len(testCases))
			}
			testMethods, methodErrors := compileTestMethods(testCases)
			testPrograms, programErrors := compileTestInvokePrograms(testCases)
			for _, test := range testCases {
				key := testCaseKey(test)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named test compilation: %v %v", methodErrors[key], programErrors[key])
				}
			}
			org := storage.NewOrgState()
			for _, name := range []string{"Account", "Contact", "Opportunity", "Task", "User"} {
				storage.EnsureStandardObject(&org, name)
			}
			oracle := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			catalog := storage.NewOrgState()
			for i := range 128 {
				name := fmt.Sprintf("Parcel%03d__c", i)
				catalog.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
			}
			workload := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &catalog})
			for _, route := range []string{"anonymous", "isTest"} {
				t.Run(route, func(t *testing.T) {
					class := "DescribeReadBinding"
					if route == "isTest" {
						class = "DescribeReadBindingTest"
					}
					var want []string
					for _, row := range rows[1:] {
						if len(row) != 3 {
							t.Fatal("invalid native row")
						}
						if row[0] == api {
							want = append(want, "ROW\t"+row[1]+"\t"+row[2])
						}
					}
					if len(want) != 9 {
						t.Fatal("missing native API observations")
					}
					for _, method := range []string{"observe", "bindGlobals"} {
						t.Run(method, func(t *testing.T) {
							program, err := vm.CompileAnonymousWithOptions(class+"."+method+"();", vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							machine := oracle.newMachine()
							if method == "bindGlobals" {
								machine = workload.newMachine()
							}
							if route == "isTest" {
								for _, test := range testCases {
									if test.MethodName == method {
										key := testCaseKey(test)
										program = testPrograms[key]
										if err := machine.RegisterMethod(testMethods[key]); err != nil {
											t.Fatal(err)
										}
									}
								}
								machine.EnableTestContext()
							}
							execute := func() {
								var result vm.Result
								if route == "isTest" {
									result, err = machine.ExecuteTestInClass(program, class)
								} else {
									result, err = machine.Execute(program)
								}
								if err != nil {
									t.Fatal(err)
								}
								if method == "observe" && !reflect.DeepEqual(result.Debug, want) {
									t.Fatalf("native=%v; local=%v", want, result.Debug)
								}
							}
							if method == "observe" {
								execute()
								return
							}
							allocations := testing.AllocsPerRun(3, execute) / 16
							// The VM byte budget separately bounds final map storage.
							// This project guard catches repeated token source probes
							// through a real method and named test invocation.
							const limit = 4096
							t.Logf("allocations/global-describe binding=%.0f budget=%d", allocations, limit)
							if allocations > limit {
								t.Fatalf("global describe binding probed token sources: %.0f allocations, budget %d", allocations, limit)
							}
						})
					}
				})
			}
		})
	}
}

package vm_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// A list built inside a class method, from anonymous Apex and from an
// @IsTest method, gives the same strict outcome whether its heap checks are
// priced or walked. The method frame also drops a record local at each body
// exit, which a priced check must not keep counting.
func TestStrictHeapPricedChecksProjectRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, heapCap := range []int{0, 40000} {
				name := api + "/" + route + "/default"
				if heapCap != 0 {
					name = api + "/" + route + "/cap"
				}
				t.Run(name, func(t *testing.T) {
					type outcome struct {
						err        string
						limits     vm.Limits
						violations []vm.LimitViolation
					}
					run := func(plansOff bool) (outcome, int) {
						defer vm.SetHeapStatementPlansOffForTest(plansOff)()
						machine, program := strictHeapProjectMachine(t, api, route)
						machine.SetLimitMode(vm.LimitModeStrict)
						if heapCap != 0 {
							caps, _ := vm.LimitCapsForProfile("default")
							caps.HeapSize = heapCap
							machine.SetLimitCaps(caps)
						}
						var result vm.Result
						var err error
						if route == "anonymous" {
							result, err = machine.Execute(program)
						} else {
							result, err = machine.ExecuteTestInClass(program, "HeapAppendWorkerTest")
						}
						out := outcome{limits: result.Limits, violations: result.LimitViolations}
						if err != nil {
							out.err = err.Error()
						}
						return out, machine.HeapChecksPricedForTest()
					}
					want, _ := run(true)
					got, priced := run(false)
					// Each of the four statements per iteration prices; a capped
					// run stops part way.
					minPriced := 1000
					if heapCap != 0 {
						minPriced = 400
					}
					if priced < minPriced {
						t.Fatalf("priced %d checks, want at least %d", priced, minPriced)
					}
					if heapCap != 0 && !strings.Contains(want.err, "Too many heapSize: ") {
						t.Fatalf("capped run err = %q, want heap LimitException", want.err)
					}
					if heapCap == 0 && want.err != "" {
						t.Fatalf("err = %q", want.err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("priced outcome differs from full walk:\n got  %+v\n want %+v", got, want)
					}
				})
			}
		}
	}
}

func strictHeapProjectMachine(t *testing.T, api, route string) (*vm.VM, ir.Program) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	const classes = "force-app/main/default/classes/"
	write(classes+"HeapAppendWorker.cls", `public class HeapAppendWorker {
    public static Integer build(Integer n) {
        List<Account> accounts = new List<Account>();
        List<Integer> marks = new List<Integer>();
        for (Integer i = 0; i < n; i++) {
            accounts.add(new Account(Name = 'a' + i));
            Account scratch = new Account(Name = 'scratch' + i);
            marks.add(i);
        }
        return Limits.getHeapSize();
    }
}`)
	write(classes+"HeapAppendWorkerTest.cls", `@IsTest private class HeapAppendWorkerTest { @IsTest static void build() { HeapAppendWorker.build(300); } }`)
	for _, name := range []string{"HeapAppendWorker", "HeapAppendWorkerTest"} {
		write(classes+name+".cls-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
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
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := apextest.OrgFromIndex(index)
	org.APIVersion = api
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
		t.Fatal(err)
	}
	source := "HeapAppendWorker.build(300);"
	if route == "isTest" {
		machine.EnableTestContext()
		if err := apextest.RegisterTestMethodForRequest(machine, index, "HeapAppendWorkerTest", "build"); err != nil {
			t.Fatal(err)
		}
		source = "HeapAppendWorkerTest.build();"
	}
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	return machine, program
}

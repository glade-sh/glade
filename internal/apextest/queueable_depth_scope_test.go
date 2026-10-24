package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These project routes drain a rejected inherited-depth child. The anonymous
// route follows the native async_stack_depth rows (inherited_one,
// overflow_flush, API 62 and 67): a System.AsyncException "Maximum stack depth
// has been reached." The isTest route drains without Test.stopTest; no row
// covers it, so it keeps the local error.
func TestQueueableInheritedDepthOutsideStopTestAnonymousAndIsTestProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, overload := range []string{"plain", "delay", "options"} {
				t.Run(api+"/"+route+"/"+overload, func(t *testing.T) {
					child := "System.enqueueJob(new Child());"
					if overload == "delay" {
						child = "System.enqueueJob(new Child(), 0);"
					} else if overload == "options" {
						child = "System.enqueueJob(new Child(), new AsyncOptions());"
					}
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
					classes := filepath.Join(root, "force-app", "main", "default", "classes")
					for name, source := range map[string]string{
						"DepthScopeProbe": `public class DepthScopeProbe {
    public class Child implements Queueable {
        public void execute(QueueableContext context) { System.assert(false, 'rejected child executed'); }
    }
    public class Parent implements Queueable {
        public void execute(QueueableContext context) { ` + child + ` }
    }
    public static void enqueue() {
        AsyncOptions options = new AsyncOptions();
        options.MaximumQueueableStackDepth = 1;
        System.enqueueJob(new Parent(), options);
    }
}`,
						"DepthScopeProbeTest": `@IsTest private class DepthScopeProbeTest {
    @IsTest static void enqueueOutsideStopTest() { DepthScopeProbe.enqueue(); }
}`,
					} {
						writeFile(t, filepath.Join(classes, name+".cls"), source)
						writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					}
					index := loadTestIndex(t, root)
					if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					machine := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).newMachine()
					source := "DepthScopeProbe.enqueue();"
					if route == "isTest" {
						cases := Discover(index, Options{SelectedClasses: []string{"DepthScopeProbeTest"}, SelectedMethod: "enqueueOutsideStopTest"})
						if len(cases) != 1 {
							t.Fatalf("named test discovery: %v", cases)
						}
						methods, failures := compileTestMethods(cases)
						key := testCaseKey(cases[0])
						if failures[key] != nil {
							t.Fatal(failures[key])
						}
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						source = "enqueueOutsideStopTest();"
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					var result vm.Result
					if route == "isTest" {
						result, err = machine.ExecuteInClass(program, "DepthScopeProbeTest")
					} else {
						result, err = machine.Execute(program)
					}
					if err != nil {
						t.Fatal(err)
					}
					err = machine.DrainAsync(&result)
					want := "MaximumQueueableStackDepth exceeded"
					if route == "anonymous" {
						want = "Maximum stack depth has been reached."
					}
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("outside stopTest project drain = %v; want %q", err, want)
					}
					if machine.HasPendingAsyncWork() {
						t.Fatal("rejected child left pending asynchronous work")
					}
				})
			}
		}
	}
}

package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// async_mixed_dml/native pins separate callbacks and same-callback rejection.
// These tests isolate dispatch and restoration; the project test replays Apex.
func TestAsyncMixedDMLTransactionRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "istest"} {
			for _, route := range []string{"Queueable", "Future", "ScheduledApex", "BatchApex", "ScheduledBatch"} {
				t.Run(api+"/"+context+"/"+route, func(t *testing.T) {
					machine := New(nil)
					org := testDataOrg()
					storage.EnsureDeterministicPlatformData(&org)
					machine.SetOrg(&org)
					if context == "istest" {
						machine.EnableTestContext()
					}
					machine.currentMethod = Method{APIVersion: api}
					if err := machine.checkMixedDML("insert", []storage.Record{{Object: "Account"}}); err != nil {
						t.Fatal(err)
					}
					parent := machine.mixedDML
					body := func(setup bool, label string) string {
						first := fmt.Sprintf("insert new PermissionSet(Name='Boundary%s', Label='Boundary %s');", label, label)
						opposite := "insert new Account(Name='rejected');"
						if !setup {
							first, opposite = "insert new Account(Name='Boundary child');", first
						}
						return first + "\nBoolean caught=false; try {" + opposite + "} catch(DmlException e) { caught=String.valueOf(e.getDmlType(0))=='MIXED_DML_OPERATION'; } System.assert(caught,'same callback rejection');"
					}
					method := func(name, source string, params ...Param) Method {
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						return Method{Name: "BoundaryWorker." + name, ClassName: "BoundaryWorker", ReturnType: "void", Params: params, Program: program, APIVersion: api}
					}
					worker := Class{Name: "BoundaryWorker", Methods: map[string]Method{}}
					job := AsyncJob{ID: "707000000000001", Kind: route, Object: Object(worker.Name)}
					switch route {
					case "Future":
						m := method("run", body(true, "Future"))
						m.IsStatic = true
						worker.Methods["run"] = m
						job.Method = m
					case "Queueable":
						worker.Interfaces = []string{"Queueable"}
						worker.Methods["execute"] = method("execute", body(true, "Queue"), Param{Name: "context", Type: "QueueableContext"})
					case "ScheduledApex":
						worker.Interfaces = []string{"Schedulable"}
						worker.Methods["execute"] = method("execute", body(true, "Schedule"), Param{Name: "context", Type: "SchedulableContext"})
					case "BatchApex", "ScheduledBatch":
						worker.Interfaces = []string{"Database.Batchable<SObject>"}
						start := method("start", body(true, "Start")+"return new List<SObject>{new Account(Name='scope')};", Param{Name: "context", Type: "Database.BatchableContext"})
						start.ReturnType = "Iterable<SObject>"
						worker.Methods["start"] = start
						worker.Methods["execute"] = method("execute", body(false, "Execute"), Param{Name: "context", Type: "Database.BatchableContext"}, Param{Name: "scope", Type: "List<SObject>"})
						worker.Methods["finish"] = method("finish", body(true, "Finish"), Param{Name: "context", Type: "Database.BatchableContext"})
					}
					if err := machine.RegisterClass(worker); err != nil {
						t.Fatal(err)
					}
					if err := machine.runAsyncJob(job, &Result{}); err != nil {
						t.Fatal(err)
					}
					if machine.testContext != nil && (machine.testContext.SetupDML || !machine.testContext.NonSetupDML) {
						t.Fatal("caller test flags changed after callback")
					}
					if !reflect.DeepEqual(machine.mixedDML, parent) {
						t.Fatalf("caller state changed: got %+v, want %+v", machine.mixedDML, parent)
					}
					if err := machine.checkMixedDML("insert", []storage.Record{{Object: "PermissionSet"}}); err == nil {
						t.Fatal("caller mixed-DML rejection lost after callback")
					}
					// A second callback of the opposite kind must also start clean.
					if route == "Queueable" || route == "Future" || route == "ScheduledApex" {
						name := "execute"
						if route == "Future" {
							name = "run"
						}
						m := worker.Methods[name]
						program, err := CompileAnonymousWithOptions(body(false, "Second"), CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						m.Program = program
						worker.Methods[name] = m
						if err := machine.RegisterClass(worker); err != nil {
							t.Fatal(err)
						}
						job.Method = m
						if err := machine.runAsyncJob(job, &Result{}); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func TestAsyncMixedDMLQueueableErrorRestoresCaller(t *testing.T) {
	for _, context := range []string{"anonymous", "istest"} {
		t.Run(context, func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			machine.SetOrg(&org)
			if context == "istest" {
				machine.EnableTestContext()
			}
			if err := machine.checkMixedDML("insert", []storage.Record{{Object: "Account"}}); err != nil {
				t.Fatal(err)
			}
			parent := machine.mixedDML
			program, err := CompileAnonymous(`insert new PermissionSet(Name='BoundaryError',Label='Boundary error'); throw new IllegalArgumentException('callback failure');`)
			if err != nil {
				t.Fatal(err)
			}
			method := Method{Name: "BoundaryError.execute", ClassName: "BoundaryError", Program: program, Params: []Param{{Name: "context", Type: "QueueableContext"}}}
			if err := machine.RegisterClass(Class{Name: "BoundaryError", Interfaces: []string{"Queueable"}, Methods: map[string]Method{"execute": method}}); err != nil {
				t.Fatal(err)
			}
			err = machine.runAsyncJob(AsyncJob{ID: "707000000000001", Kind: "Queueable", Object: Object("BoundaryError")}, &Result{})
			if err == nil || !strings.Contains(err.Error(), "callback failure") {
				t.Fatalf("expected callback failure after setup DML, got %v", err)
			}
			if machine.mixedDML != parent {
				t.Fatalf("caller state changed after failure: %+v", machine.mixedDML)
			}
			if machine.testContext != nil && (machine.testContext.SetupDML || !machine.testContext.NonSetupDML) {
				t.Fatal("caller test flags changed after failure")
			}
		})
	}
}

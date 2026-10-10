package vm

import (
	"strings"
	"testing"
)

func TestSynchronousActionBoundaryRejectsFutureMethod(t *testing.T) {
	program, err := CompileAnonymous(`return 'future';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "FutureWorker.run",
		ClassName:  "FutureWorker",
		ReturnType: "String",
		IsStatic:   true,
		Modifiers:  []string{"AuraEnabled", "future"},
		Program:    program,
	}); err != nil {
		t.Fatal(err)
	}
	machine.SetSynchronousActionBoundary(true)
	out, err := machine.InvokeLWCMethod("FutureWorker", "run", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Success || out.Error == nil || out.Error.Type != "UnsupportedFeature" || !strings.Contains(out.Error.Message, "synchronous LWC action boundary") {
		t.Fatalf("result = %#v", out)
	}
}

func TestSynchronousActionBoundaryRejectsEventBusPublish(t *testing.T) {
	program, err := CompileAnonymous(`EventBus.publish(null); return 'event';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "EventWorker.publish",
		ClassName:  "EventWorker",
		ReturnType: "String",
		IsStatic:   true,
		Modifiers:  []string{"AuraEnabled"},
		Program:    program,
	}); err != nil {
		t.Fatal(err)
	}
	machine.SetSynchronousActionBoundary(true)
	out, err := machine.InvokeLWCMethod("EventWorker", "publish", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Success || out.Error == nil || out.Error.Type != "UnsupportedFeature" || !strings.Contains(out.Error.Message, "synchronous LWC action boundary") {
		t.Fatalf("result = %#v", out)
	}
}

func TestSynchronousActionBoundaryRejectsDatabaseAsyncDML(t *testing.T) {
	program, err := CompileAnonymous(`Database.insertAsync(new Account(Name = 'async')); return 'async';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "AsyncDMLWorker.run",
		ClassName:  "AsyncDMLWorker",
		ReturnType: "String",
		IsStatic:   true,
		Modifiers:  []string{"AuraEnabled"},
		Program:    program,
	}); err != nil {
		t.Fatal(err)
	}
	machine.SetSynchronousActionBoundary(true)
	out, err := machine.InvokeLWCMethod("AsyncDMLWorker", "run", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Success || out.Error == nil || out.Error.Type != "UnsupportedFeature" || !strings.Contains(out.Error.Message, "Database.insertAsync") {
		t.Fatalf("result = %#v", out)
	}
	if !machine.HasRejectedAsyncAction() {
		t.Fatal("Database.insertAsync violation was not recorded")
	}
}

func TestSynchronousActionBoundaryRecordsAndClearsViolation(t *testing.T) {
	machine := New(nil)
	machine.SetSynchronousActionBoundary(true)
	if err := machine.rejectSynchronousAsyncAction("test violation"); err == nil {
		t.Fatal("expected rejection error")
	}
	if !machine.HasRejectedAsyncAction() {
		t.Fatal("violation was not recorded")
	}
	machine.SetSynchronousActionBoundary(false)
	if machine.HasRejectedAsyncAction() {
		t.Fatal("violation was not cleared when boundary was disabled")
	}
}

package vm

import (
	"strings"
	"testing"
)

func TestSynchronousActionBoundaryDefersFutureMethod(t *testing.T) {
	work, err := CompileAnonymous(`return 'future';`)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := CompileAnonymous(`FutureWorker.work(); return 'queued';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "FutureWorker.work", ClassName: "FutureWorker", ReturnType: "String", IsStatic: true, Modifiers: []string{"future"}, Program: work},
		{Name: "FutureWorker.run", ClassName: "FutureWorker", ReturnType: "String", IsStatic: true, Modifiers: []string{"AuraEnabled"}, Program: entry},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	machine.SetSynchronousActionBoundary(true)
	out, err := machine.InvokeLWCMethod("FutureWorker", "run", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Superseded by the lightning_action_async_drain captures: a future called
	// by an action is queued for the post-commit drain, not rejected.
	if !out.Success || out.Error != nil || !machine.HasPendingAsyncWork() || machine.HasRejectedAsyncAction() {
		t.Fatalf("result = %#v pending=%v", out, machine.HasPendingAsyncWork())
	}
}

func TestSynchronousActionBoundaryDoesNotRejectEventBusPublish(t *testing.T) {
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
	// EventBus.publish is ordinary platform-event publication inside an action;
	// the invalid argument fails on its own terms, not as a boundary violation.
	if out.Error != nil && strings.Contains(out.Error.Message, "synchronous LWC action boundary") || machine.HasRejectedAsyncAction() {
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

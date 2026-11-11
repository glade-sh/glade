package vm

import "testing"

func runWorkProgram(t *testing.T, machine *VM, source string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestWorkCountsLoopsAndRows(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.ResetLimits()
	runWorkProgram(t, machine, `
		Integer n = 0;
		for (Integer i = 0; i < 7; i++) { n++; }
		while (n < 10) { n++; }
		List<Account> rows = new List<Account>{ new Account(Name='a'), new Account(Name='b') };
		insert rows;
		List<Account> found = [SELECT Id FROM Account];`)
	got := machine.WorkCounts()
	if got.LoopIterations < 10 {
		t.Fatalf("loop iterations = %d, want at least 10", got.LoopIterations)
	}
	if got.Instructions == 0 || got.DMLRows != 2 || got.DMLStatements != 1 || got.SOQLQueries != 1 || got.SOQLRows != 2 {
		t.Fatalf("work = %+v", got)
	}
	machine.ResetLimits()
	if reset := machine.WorkCounts(); reset != (WorkCounts{}) {
		t.Fatalf("ResetLimits left work %+v", reset)
	}
}

func TestWorkCountsMethodAttributionOnlyWhenTracking(t *testing.T) {
	build := func(tracking bool) *VM {
		machine := New(nil)
		machine.SetWorkTracking(tracking)
		body, err := CompileAnonymousWithOptions(`Integer n = 0; for (Integer i = 0; i < 5; i++) { n++; }`, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		err = machine.RegisterClass(Class{Name: "WorkHelper", Methods: map[string]Method{"spin": {Name: "WorkHelper.spin", ClassName: "WorkHelper", ReturnType: "void", IsStatic: true, Program: body}}})
		if err != nil {
			t.Fatal(err)
		}
		machine.ResetLimits()
		runWorkProgram(t, machine, `WorkHelper.spin(); WorkHelper.spin();`)
		return machine
	}
	if methods := build(false).MethodWork(); len(methods) != 0 {
		t.Fatalf("methods tracked while disabled: %+v", methods)
	}
	methods := build(true).MethodWork()
	if len(methods) != 1 || methods[0].Method != "WorkHelper.spin" || methods[0].Calls != 2 || methods[0].Work.LoopIterations < 10 {
		t.Fatalf("methods = %+v", methods)
	}
}

// The advisory counters must not change the CPU model.
func TestWorkTrackingDoesNotChangeCPUTime(t *testing.T) {
	cpu := func(tracking bool) int {
		machine := New(nil)
		machine.SetWorkTracking(tracking)
		machine.ResetLimits()
		runWorkProgram(t, machine, `Integer n = 0; for (Integer i = 0; i < 500; i++) { n++; }`)
		return machine.cpuBudgetUsed
	}
	if off, on := cpu(false), cpu(true); off != on {
		t.Fatalf("cpu budget with tracking = %d, without = %d", on, off)
	}
}

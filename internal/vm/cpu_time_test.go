package vm

import (
	"io"
	"testing"
	"time"
)

type cpuTimeWriter func([]byte) (int, error)

func (w cpuTimeWriter) Write(p []byte) (int, error) { return w(p) }

// CPU time excludes waits, including time when unrelated work runs in the same
// process. Native wait rows: testdata/conformance/cpu_time/native/api{62,67}/org.tsv.
func TestCPUTimeStableUnderConcurrentLoad(t *testing.T) {
	program, err := CompileAnonymous(`Integer before = Limits.getCpuTime(); System.debug('pause'); Integer after = Limits.getCpuTime(); System.assertEquals(10000, Limits.getLimitCpuTime());`)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := New(io.Discard).Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	base := baseline.Vars["after"].Int - baseline.Vars["before"].Int
	requests, stopped := make(chan chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		for done := range requests {
			until := time.Now().Add(80 * time.Millisecond)
			for time.Now().Before(until) {
			} // unrelated CPU load while Apex waits
			close(done)
		}
	}()
	defer func() { close(requests); <-stopped }()
	writer := cpuTimeWriter(func(p []byte) (int, error) {
		done := make(chan struct{})
		requests <- done
		<-done
		return len(p), nil
	})
	for i := 0; i < 50; i++ {
		result, err := New(writer).Execute(program)
		if err != nil {
			t.Fatal(err)
		}
		used := result.Vars["after"].Int - result.Vars["before"].Int
		if used != base || result.Limits.CPUTimeMS != baseline.Limits.CPUTimeMS {
			t.Fatalf("run %d: CPU delta under concurrent load = %dms, unloaded = %dms", i, used, base)
		}
	}
}

func TestCPUTimeExcludesIdleBetweenExecutions(t *testing.T) {
	program, err := CompileAnonymous(`Integer sample = Limits.getCpuTime();`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	first, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	before := first.Limits.CPUTimeMS
	time.Sleep(80 * time.Millisecond)
	second, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if second.Limits.CPUTimeMS < before || second.Limits.CPUTimeMS > before+15 {
		t.Fatalf("CPU samples around idle = %d, %d", before, second.Limits.CPUTimeMS)
	}
}

// This is the local deterministic cost contract, not a claim that a native
// millisecond equals a fixed number of Apex operations.
func TestCPUTimeAccountsForApexWork(t *testing.T) {
	program, err := CompileAnonymous(`Integer before = Limits.getCpuTime(); Integer sum = 0; for(Integer i=0;i<2000;i++){sum += i;} Integer after = Limits.getCpuTime();`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if result.Vars["after"].Int <= result.Vars["before"].Int {
		t.Fatal("Apex work did not advance CPU time")
	}
	if result.Limits.CPUTimeMS != machine.cpuBudgetUsed/cpuBudgetUnitsPerMillisecond {
		t.Fatal("CPU sample does not match operation estimate")
	}
}

func TestCPUTimeAsyncWindowRestoresParent(t *testing.T) {
	machine := New(nil)
	if err := machine.incrementCPUBudget(2000); err != nil {
		t.Fatal(err)
	}
	before := machine.sampleCPUTimeMS()
	program, err := CompileAnonymous(`Integer sum=0;for(Integer i=0;i<2000;i++){sum+=i;}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.withAsyncLimitWindow(func() (Value, error) {
		if got := machine.sampleCPUTimeMS(); got != 0 {
			t.Fatalf("fresh child CPU=%d", got)
		}
		result, err := machine.Execute(program)
		if err != nil {
			return Null, err
		}
		if result.Limits.CPUTimeMS <= before {
			t.Fatal("child work was not charged")
		}
		return Null, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := machine.sampleCPUTimeMS(); got != before {
		t.Fatalf("parent charged for child CPU: before=%d after=%d", before, got)
	}
	if machine.cpuBudgetUsed != 2000 {
		t.Fatalf("parent budget=%d", machine.cpuBudgetUsed)
	}
}

func TestCPUTimeDirectEntryRoutes(t *testing.T) {
	for _, route := range []string{"static", "lwc", "aura", "vf", "vf-controller", "async-drain"} {
		t.Run(route, func(t *testing.T) {
			budget := `Integer until = Limits.getCpuTime() + 80; while (Limits.getCpuTime() < until) {} System.assert(Limits.getCpuTime() >= until);`
			body, err := CompileAnonymous(budget)
			if err != nil {
				t.Fatal(err)
			}
			action, err := CompileAnonymous(budget + ` return new PageReference('/apex/Cpu');`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			org := testDataOrg()
			machine.SetOrg(&org)
			class := Class{Name: "CpuEntry", Interfaces: []string{"Queueable"}, Methods: map[string]Method{
				"run":     {Name: "CpuEntry.run", ClassName: "CpuEntry", ReturnType: "void", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: body},
				"action":  {Name: "CpuEntry.action", ClassName: "CpuEntry", ReturnType: "PageReference", Modifiers: []string{"public"}, Program: action},
				"execute": {Name: "CpuEntry.execute", ClassName: "CpuEntry", ReturnType: "void", Modifiers: []string{"public"}, Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: body},
			}}
			if err := machine.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			var ui UIInvocationResult
			switch route {
			case "static":
				_, err = machine.CallStatic("CpuEntry.run", nil)
			case "lwc":
				ui, err = machine.InvokeLWCMethod("CpuEntry", "run", nil)
			case "aura":
				ui, err = machine.InvokeAuraAction("CpuEntry", "run", nil)
			case "vf":
				ui, err = machine.InvokeVisualforceAction("CpuEntry", "action", "/apex/Cpu", nil)
			case "vf-controller":
				_, _, ui, err = machine.InvokeVisualforceActionOnController(Object("CpuEntry"), "CpuEntry", "action", "/apex/Cpu", nil)
			case "async-drain":
				start, compileErr := CompileAnonymous(`System.enqueueJob(new CpuEntry());`)
				if compileErr != nil {
					t.Fatal(compileErr)
				}
				result, runErr := machine.Execute(start)
				if runErr != nil {
					t.Fatal(runErr)
				}
				err = machine.DrainAsync(&result)
			}
			if err != nil {
				t.Fatal(err)
			}
			if route != "static" && route != "async-drain" && !ui.Success {
				t.Fatalf("entry failed: %+v", ui.Error)
			}
			if machine.sampleCPUTimeMS() == 0 {
				t.Fatal("entry did not charge CPU work")
			}
		})
	}
}

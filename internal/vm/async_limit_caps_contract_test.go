package vm

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// These are local configuration contracts. They do not assert Salesforce CPU
// accounting or a new native governor limit.
func TestAsyncEntryLimitCapsContracts(t *testing.T) {
	for _, route := range []string{"Queueable", "future", "Batch", "Scheduled", "ScheduledBatch"} {
		for _, explicit := range []bool{false, true} {
			name := "native"
			if explicit {
				name = "explicit-defaults"
			}
			t.Run(route+"/"+name, func(t *testing.T) {
				machine := New(nil)
				want := defaultLimitCaps()
				if explicit {
					caps, ok := LimitCapsForProfile("strict-sync")
					if !ok {
						t.Fatal("strict-sync profile missing")
					}
					machine.SetLimitCaps(caps)
				} else {
					want.Queries = 200
					if route != "Scheduled" {
						want.CPUTimeMS = 60000
						want.HeapSize = asyncHeapLimit
					}
				}
				runAsyncCapsContract(t, machine, route, want, true)
			})
		}
	}
	for _, tc := range []struct {
		name         string
		queries, cpu int
	}{
		{"narrower", 7, 2500},
		{"larger", 300, 70000},
	} {
		t.Run("Queueable/explicit-"+tc.name, func(t *testing.T) {
			machine := New(nil)
			caps := defaultLimitCaps()
			caps.Queries, caps.CPUTimeMS = tc.queries, tc.cpu
			machine.SetLimitCaps(caps)
			runAsyncCapsContract(t, machine, "Queueable", caps, true)
		})
	}
}

func TestLocalAsyncDrainLimitCapsContracts(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "native"
		if explicit {
			name = "explicit-defaults"
		}
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			caps := defaultLimitCaps()
			if explicit {
				machine.SetLimitCaps(caps)
			} else {
				caps.Queries, caps.CPUTimeMS, caps.HeapSize = 200, 60000, asyncHeapLimit
			}
			runAsyncCapsContract(t, machine, "Queueable", caps, false)
		})
	}
}

func runAsyncCapsContract(t *testing.T, machine *VM, route string, want LimitCaps, testContext bool) {
	t.Helper()
	checks := fmt.Sprintf(`System.assertEquals(%d, Limits.getLimitQueries());
System.assertEquals(%d, Limits.getLimitCpuTime());
System.assertEquals(%d, Limits.getLimitHeapSize());
System.assertEquals(0, Limits.getQueries());
System.assertEquals(0, [SELECT COUNT() FROM Account]);
System.assertEquals(1, Limits.getQueries());
System.debug('caps checked');`, want.Queries, want.CPUTimeMS, want.HeapSize)
	worker, err := CompileAnonymous(checks)
	if err != nil {
		t.Fatal(err)
	}
	class := Class{Name: "CapsWorker", Methods: map[string]Method{}}
	start := ""
	wantCallbacks := 1
	switch route {
	case "Queueable":
		class.Interfaces = []string{"Queueable"}
		class.Methods["execute"] = Method{Name: "CapsWorker.execute", ClassName: class.Name, ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: worker}
		start = "System.enqueueJob(new CapsWorker());"
	case "future":
		class.Methods["run"] = Method{Name: "CapsWorker.run", ClassName: class.Name, ReturnType: "void", IsStatic: true, Modifiers: []string{"future"}, Program: worker}
		start = "CapsWorker.run();"
	case "Scheduled":
		class.Interfaces = []string{"Schedulable"}
		class.Methods["execute"] = Method{Name: "CapsWorker.execute", ClassName: class.Name, ReturnType: "void", Params: []Param{{Name: "context", Type: "SchedulableContext"}}, Program: worker}
		start = "System.schedule('caps', '0 0 0 1 1 ? 2099', new CapsWorker());"
	case "Batch", "ScheduledBatch":
		class.Interfaces = []string{"Database.Batchable<SObject>"}
		batchStart, err := CompileAnonymous(checks + "\nreturn new List<SObject>{new Account(Name = 'one'), new Account(Name = 'two')};")
		if err != nil {
			t.Fatal(err)
		}
		class.Methods["start"] = Method{Name: "CapsWorker.start", ClassName: class.Name, ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: batchStart}
		class.Methods["execute"] = Method{Name: "CapsWorker.execute", ClassName: class.Name, ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: worker}
		class.Methods["finish"] = Method{Name: "CapsWorker.finish", ClassName: class.Name, ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: worker}
		start = "Database.executeBatch(new CapsWorker(), 1);"
		if route == "ScheduledBatch" {
			start = "System.scheduleBatch(new CapsWorker(), 'caps', 1, 1);"
		}
		wantCallbacks = 4
	default:
		t.Fatalf("unknown route %q", route)
	}
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.SetLimitMode(LimitModeStrict)
	if err := machine.RegisterClass(class); err != nil {
		t.Fatal(err)
	}
	parentCaps := machine.limitCaps
	if testContext {
		machine.EnableTestContext()
		start = "Test.startTest();\n" + start + "\nTest.stopTest();"
	}
	start = "System.assertEquals(0, [SELECT COUNT() FROM Account]);\n" + start + "\nSystem.assertEquals(1, Limits.getQueries());"
	program, err := CompileAnonymous(start)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	if err == nil && !testContext {
		err = machine.DrainAsync(&result)
	}
	if err != nil {
		t.Fatalf("%s entry: %v", route, err)
	}
	if len(result.Debug) != wantCallbacks {
		t.Fatalf("callbacks = %v, want %d checks", result.Debug, wantCallbacks)
	}
	if machine.limits.Queries != 1 {
		t.Fatalf("parent queries = %d, want 1", machine.limits.Queries)
	}
	if machine.limitCaps != parentCaps {
		t.Fatalf("parent caps = %#v, want %#v", machine.limitCaps, parentCaps)
	}
}

func TestAsyncWindowExplicitEnforcementAndRestorationContracts(t *testing.T) {
	for _, counter := range []string{"queries", "cpuTime"} {
		for _, strict := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/strict=%t", counter, strict), func(t *testing.T) {
				machine := New(nil)
				caps := defaultLimitCaps()
				machine.SetLimitCaps(caps)
				if strict {
					machine.SetLimitMode(LimitModeStrict)
				}
				machine.limits = Limits{Queries: 3, CPUTimeMS: 9}
				machine.limitViolations = []LimitViolation{{Name: "queryRows", Used: 2, Limit: 1}}
				machine.cpuBudgetUsed = 8
				parentLimits, parentCaps := machine.limits, machine.limitCaps
				parentViolations := append([]LimitViolation(nil), machine.limitViolations...)
				_, err := machine.withAsyncLimitWindow(func() (Value, error) {
					if machine.limits != (Limits{}) || machine.cpuBudgetUsed != 0 {
						t.Fatal("child counters did not reset")
					}
					cap := caps.Queries
					unitsPerCount := 1
					if counter == "cpuTime" {
						cap = caps.CPUTimeMS
						unitsPerCount = cpuBudgetUnitsPerMillisecond
					}
					if err := machine.incrementLimit(counter, cap*unitsPerCount); err != nil {
						t.Fatalf("at cap: %v", err)
					}
					limitErr := machine.incrementLimit(counter, unitsPerCount)
					want := []LimitViolation{{Name: counter, Used: cap + 1, Limit: cap}}
					if !reflect.DeepEqual(machine.limitViolations, want) {
						t.Errorf("child violations = %#v, want %#v", machine.limitViolations, want)
					}
					return Null, limitErr
				})
				var runtimeErr *RuntimeError
				if strict && (!errors.As(err, &runtimeErr) || runtimeErr.Type != "System.LimitException") {
					t.Errorf("strict error = %v, want LimitException", err)
				} else if !strict && err != nil {
					t.Errorf("permissive error = %v", err)
				}
				if machine.limits != parentLimits || machine.limitCaps != parentCaps || !reflect.DeepEqual(machine.limitViolations, parentViolations) || machine.cpuBudgetUsed != 8 {
					t.Fatalf("parent limit window was not restored")
				}
			})
		}
	}
}

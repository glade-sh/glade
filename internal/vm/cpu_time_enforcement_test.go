package vm

import (
	"errors"
	"fmt"
	"testing"
)

// Native work rows exceed 20 ms without exhausting the 10000 ms cap:
// testdata/conformance/cpu_time/native/api{62,67}/org.tsv, work.
// Async caps and uncatchable exhaustion are recorded in async/exhaustion.
// Outside a test context glade runs a future method inline and never runs a
// far-future schedule in a local drain, so those two routes execute between
// Test.startTest and Test.stopTest, where they get their own async window.
func TestCPUTimeStrictBudgetRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "Queueable", "future", "Scheduled", "Batch", "ScheduledBatch"} {
			for _, explicit := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/explicit=%t", api, route, explicit), func(t *testing.T) {
					machine := New(nil)
					machine.SetLimitMode(LimitModeStrict)
					wantCap, budget := 10000, 20
					if route != "anonymous" && route != "Scheduled" {
						wantCap, budget = 60000, 80
					}
					if explicit {
						caps := defaultLimitCaps()
						caps.CPUTimeMS = 40
						machine.SetLimitCaps(caps)
						wantCap, budget = 40, 20
					}
					checks := fmt.Sprintf(`System.assertEquals(%d, Limits.getLimitCpuTime());
Integer until = Limits.getCpuTime() + %d;
while (Limits.getCpuTime() < until) {}
System.assert(Limits.getCpuTime() >= until);
System.assert(Limits.getCpuTime() < Limits.getLimitCpuTime());
System.debug('budget completed');`, wantCap, budget)
					worker, err := CompileAnonymousWithOptions(checks, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if route == "anonymous" {
						result, err := machine.Execute(worker)
						if err != nil || len(result.LimitViolations) != 0 {
							t.Fatalf("budget loop: %v, violations=%v", err, result.LimitViolations)
						}
						return
					}
					class := Class{Name: "CpuBudgetWorker", Methods: map[string]Method{}}
					execute := Method{Name: "CpuBudgetWorker.execute", ClassName: class.Name, ReturnType: "void", Program: worker}
					start, callbacks := "", 1
					switch route {
					case "Queueable":
						class.Interfaces = []string{"Queueable"}
						execute.Params = []Param{{Name: "context", Type: "QueueableContext"}}
						start = "System.enqueueJob(new CpuBudgetWorker());"
					case "future":
						execute.IsStatic, execute.Modifiers = true, []string{"future"}
						start = "CpuBudgetWorker.execute();"
					case "Scheduled":
						class.Interfaces = []string{"Schedulable"}
						execute.Params = []Param{{Name: "context", Type: "SchedulableContext"}}
						start = "System.schedule('cpu', '0 0 0 1 1 ? 2099', new CpuBudgetWorker());"
					case "Batch", "ScheduledBatch":
						class.Interfaces = []string{"Database.Batchable<SObject>"}
						execute.Params = []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}
						batchStart, err := CompileAnonymousWithOptions(checks+" return new List<SObject>{new Account(Name='one')};", CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						params := []Param{{Name: "context", Type: "Database.BatchableContext"}}
						class.Methods["start"] = Method{Name: "CpuBudgetWorker.start", ClassName: class.Name, ReturnType: "Iterable<SObject>", Params: params, Program: batchStart}
						class.Methods["finish"] = Method{Name: "CpuBudgetWorker.finish", ClassName: class.Name, ReturnType: "void", Params: params, Program: worker}
						start, callbacks = "Database.executeBatch(new CpuBudgetWorker(), 1);", 3
						if route == "ScheduledBatch" {
							start = "System.scheduleBatch(new CpuBudgetWorker(), 'cpu', 1, 1);"
						}
					}
					class.Methods["execute"] = execute
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
					org := testDataOrg()
					machine.SetOrg(&org)
					testContext := route == "future" || route == "Scheduled"
					if testContext {
						machine.EnableTestContext()
						start = "Test.startTest();\n" + start + "\nTest.stopTest();"
					}
					program, err := CompileAnonymousWithOptions(start, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err == nil && !testContext {
						err = machine.DrainAsync(&result)
					}
					if err != nil || len(result.Debug) != callbacks {
						t.Fatalf("budget callbacks=%v, want %d; error=%v", result.Debug, callbacks, err)
					}
				})
			}
		}
	}
}

func TestCPUTimeEnforcementUsesReportedMilliseconds(t *testing.T) {
	for _, cap := range []int{1, 40, 10000, 60000} {
		for _, mode := range []LimitMode{LimitModeStrict, LimitModePermissive} {
			t.Run(fmt.Sprintf("cap=%d/%s", cap, mode), func(t *testing.T) {
				machine := New(nil)
				machine.SetLimitMode(mode)
				caps := defaultLimitCaps()
				caps.CPUTimeMS = cap
				machine.SetLimitCaps(caps)
				if err := machine.incrementCPUBudget(cap * cpuBudgetUnitsPerMillisecond); err != nil {
					t.Fatalf("at reported cap: %v", err)
				}
				if err := machine.incrementCPUBudget(cpuBudgetUnitsPerMillisecond - 1); err != nil || len(machine.limitViolations) != 0 || machine.sampleCPUTimeMS() != cap {
					t.Fatalf("fractional work below next ms: %v, sample=%d, violations=%v", err, machine.sampleCPUTimeMS(), machine.limitViolations)
				}
				err := machine.incrementCPUBudget(1)
				var limitErr *RuntimeError
				if mode == LimitModeStrict && (!errors.As(err, &limitErr) || limitErr.Type != "System.LimitException") {
					t.Fatalf("past reported cap: %v", err)
				}
				if mode == LimitModePermissive && err != nil {
					t.Fatal(err)
				}
				if got := machine.limitViolations; len(got) != 1 || got[0] != (LimitViolation{Name: "cpuTime", Used: cap + 1, Limit: cap}) || machine.sampleCPUTimeMS() != cap+1 {
					t.Fatalf("reported CPU=%d, violations=%v", machine.sampleCPUTimeMS(), got)
				}
			})
		}
	}
	for _, cap := range []int{0, -1} {
		t.Run(fmt.Sprintf("cap=%d", cap), func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			caps := defaultLimitCaps()
			caps.CPUTimeMS = cap
			machine.SetLimitCaps(caps)
			err := machine.incrementCPUBudget(1)
			if (err != nil) != (cap == 0) {
				t.Fatalf("first work unit with cap %d: %v", cap, err)
			}
			if cap == -1 {
				if err := machine.incrementCPUBudget(60001 * cpuBudgetUnitsPerMillisecond); err != nil || len(machine.limitViolations) != 0 {
					t.Fatalf("disabled cap: %v, violations=%v", err, machine.limitViolations)
				}
			}
		})
	}
}

func TestCPUTimeStrictExhaustionIsUncatchable(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			// Start at the boundary to exercise the default cap without a long spin.
			machine.cpuBudgetUsed = machine.limitCaps.CPUTimeMS * cpuBudgetUnitsPerMillisecond
			program, err := CompileAnonymousWithOptions(`Boolean caught = false; try { while (true) {} } catch (Exception e) { caught = true; }`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			var limitErr *RuntimeError
			if !errors.As(err, &limitErr) || limitErr.Type != "System.LimitException" || result.Vars["caught"].Bool {
				t.Fatalf("exhaustion was not an uncatchable LimitException: %v", err)
			}
			if result.Limits.CPUTimeMS != 10001 || len(result.LimitViolations) != 1 || result.LimitViolations[0].Used != result.Limits.CPUTimeMS {
				t.Fatalf("exhaustion sample=%d violations=%v", result.Limits.CPUTimeMS, result.LimitViolations)
			}
		})
	}
}

func TestCPUTimeEmptyLoopFormsAdvance(t *testing.T) {
	for _, loop := range []string{
		`while (Limits.getCpuTime() < until) {}`,
		`do {} while (Limits.getCpuTime() < until);`,
		`for (; Limits.getCpuTime() < until;) {}`,
	} {
		t.Run(loop, func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			program, err := CompileAnonymous(`Integer until = Limits.getCpuTime() + 20; ` + loop + ` System.assert(Limits.getCpuTime() >= until);`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, expression := range []string{"values", "values.iterator()"} {
		t.Run(expression, func(t *testing.T) {
			machine := New(nil)
			values := make([]Value, 3000)
			for i := range values {
				values[i] = Int(int64(i))
			}
			list := typedList("List<Integer>")
			list.List = values
			machine.Globals["values"] = list
			machine.VarTypes["values"] = "List<Integer>"
			program, err := CompileAnonymous(`Integer before = Limits.getCpuTime(); for (Integer value : ` + expression + `) {} Integer after = Limits.getCpuTime();`)
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil || result.Vars["after"].Int <= result.Vars["before"].Int {
				t.Fatalf("empty traversal did not charge work: %v, CPU %d -> %d", err, result.Vars["before"].Int, result.Vars["after"].Int)
			}
		})
	}
}

func TestCPUTimeStrictLoopExhaustionPrecedesIterationGuard(t *testing.T) {
	for _, loop := range []string{`for (;;) {}`, `do {} while (true);`} {
		t.Run(loop, func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			// Leave enough CPU for the old generic iteration guard to be reached.
			machine.cpuBudgetUsed = machine.limitCaps.CPUTimeMS*cpuBudgetUnitsPerMillisecond - maxLoopIterations - cpuBudgetUnitsPerMillisecond
			program, err := CompileAnonymous(loop)
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			var limitErr *RuntimeError
			if !errors.As(err, &limitErr) || limitErr.Type != "System.LimitException" || result.Limits.CPUTimeMS != 10001 {
				t.Fatalf("strict loop must exhaust reported CPU: %v, sample=%d", err, result.Limits.CPUTimeMS)
			}
		})
	}
}

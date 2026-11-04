package vm

import (
	"github.com/glade-sh/glade/internal/ir"
	"testing"
)

func TestMemoryPoppedSlotsAfterBodyAndPanic(t *testing.T) {
	for _, isTrigger := range []bool{false, true} {
		for _, panics := range []bool{false, true} {
			machine := New(nil)
			caller := map[string]Value{"keep": String("caller")}
			outer := map[string]Value{"outer": String("outer")}
			dead := map[string]Value{"dead": List(String("retained graph"))}
			machine.Globals = caller
			machine.scopeStack = make([]map[string]Value, 1, 2)
			machine.scopeStack[0] = outer
			machine.scopeStack[:2][1] = dead
			machine.callStack = make([]callFrame, 1, 2)
			machine.callStack[0] = callFrame{Symbol: "outer", File: "outer.cls"}
			machine.callStack[:2][1] = callFrame{Symbol: "dead", File: "dead.cls", invocableFlowOutput: new(invocableFlowOutputObservation)}
			program := ir.Program{Instructions: []ir.Instruction{{Op: ir.OpReturn}}}
			if panics {
				machine.ctx = nil
				machine.contextPollCount = 63
			}
			var caught any
			func() {
				defer func() { caught = recover() }()
				if isTrigger {
					_, err := machine.runTrigger(Trigger{Name: "MemoryTrigger", Object: "Account", Timing: "after", Operation: "insert", Program: program}, nil, nil, &Result{})
					if err != nil {
						t.Errorf("trigger: %v", err)
					}
				} else {
					value, err := machine.callMethodWithReceiver(Method{Name: "memoryProbe", IsStatic: true, Program: program}, Null, nil, &Result{})
					if err != nil || value.Kind != ValueNull {
						t.Errorf("method: value=%#v err=%v", value, err)
					}
				}
			}()
			if (caught != nil) != panics {
				t.Fatalf("trigger=%v panic=%v caught=%v", isTrigger, panics, caught)
			}
			if !panics && machine.contextPollCount != 1 {
				t.Fatal("method/trigger body did not execute")
			}
			if len(machine.callStack) != 1 || machine.callStack[0].Symbol != "outer" {
				t.Fatal("active call frame changed")
			}
			if machine.callStack[:2][1] != (callFrame{}) {
				t.Fatal("popped frame still retained")
			}
			if machine.Globals["keep"].Text != "caller" {
				t.Fatal("caller not restored")
			}
			machine.Globals["mutated"] = String("visible")
			if caller["mutated"].Text != "visible" {
				t.Fatal("caller identity changed")
			}
			if len(machine.scopeStack) != 1 || machine.scopeStack[0]["outer"].Text != "outer" {
				t.Fatal("active scope changed")
			}
			if !isTrigger && machine.scopeStack[:2][1] != nil {
				t.Fatal("popped scope still retained")
			}
			if isTrigger && machine.scopeStack[:2][1]["dead"].List[0].Text != "retained graph" {
				t.Fatal("trigger changed a scope slot it never pushed")
			}
		}
	}
}

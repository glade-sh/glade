package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestInvocableFlowTargetReferenceValidation(t *testing.T) {
	const count = 80
	targets := make(map[string][]storage.FlowStep, count)
	for i := 0; i < count; i++ {
		steps := []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Operator: "Assign"}}}
		if i+1 < count {
			ref := storage.FlowStep{Kind: "reference", Target: fmt.Sprintf("step%d", i+1)}
			steps = append(steps, storage.FlowStep{Kind: "decision", Branches: []storage.FlowBranch{{Steps: []storage.FlowStep{ref}}, {Default: true, Steps: []storage.FlowStep{ref}}}})
		}
		targets[fmt.Sprintf("step%d", i)] = steps
	}
	rule := storage.FlowRule{
		ProcessType: "AutoLaunchedFlow", Variables: []storage.FlowVariable{{Name: "approved", DataType: "Boolean"}},
		Steps: []storage.FlowStep{{Kind: "reference", Target: "step0"}}, StepTargets: targets,
	}
	if !invocableFlowActionSupported(rule) {
		t.Fatal("supported shared DAG rejected")
	}
	for _, test := range []struct {
		name string
		step storage.FlowStep
	}{
		{name: "wrong assignment", step: storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{Operator: "Add"}}},
		{name: "wrong object", step: storage.FlowStep{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{ObjectName: "Contact"}}},
		{name: "fault", step: storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{Operator: "Assign"}, FaultTarget: "recover"}},
		{name: "cycle", step: storage.FlowStep{Kind: "reference", Target: "step0"}},
		{name: "missing", step: storage.FlowStep{Kind: "reference", Target: "absent"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			targets["step79"] = []storage.FlowStep{test.step}
			if invocableFlowActionSupported(rule) {
				t.Fatalf("unsupported referenced %s accepted", test.name)
			}
		})
	}
	if !invocableFlowActionStepsSupported([]storage.FlowStep{{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{ObjectName: "Account"}}}) {
		t.Fatal("legacy Account create changed")
	}
	if invocableFlowActionStepsSupported([]storage.FlowStep{{Kind: "loop"}}) {
		t.Fatal("legacy loop boundary changed")
	}
}

// Native Invocable.Action rows (apextest/testdata/conformance/flow_null_comparisons
// routes, API 62/67) run a flow with internal Currency variables. Currency
// inputs and outputs have no native action rows and keep the boundary.
func TestInvocableFlowActionCurrencyVariableAdmission(t *testing.T) {
	steps := []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Operator: "Assign"}}}
	for _, test := range []struct {
		name     string
		variable storage.FlowVariable
		want     bool
	}{
		{name: "internal", variable: storage.FlowVariable{Name: "price", DataType: "Currency"}, want: true},
		{name: "input", variable: storage.FlowVariable{Name: "price", DataType: "Currency", IsInput: true}},
		{name: "output", variable: storage.FlowVariable{Name: "price", DataType: "Currency", IsOutput: true}},
		{name: "collection", variable: storage.FlowVariable{Name: "price", DataType: "Currency", IsCollection: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rule := storage.FlowRule{ProcessType: "AutoLaunchedFlow", Variables: []storage.FlowVariable{test.variable}, Steps: steps}
			if got := invocableFlowActionSupported(rule); got != test.want {
				t.Fatalf("supported=%v; want %v", got, test.want)
			}
		})
	}
}

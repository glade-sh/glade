package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native rows cover only record-triggered before-save decisions and self/invalid ID operands.
func TestR6FlowUncapturedGlobalDecisionRoutes(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	branch := storage.FlowBranch{Name: "Global", Criteria: []storage.WorkflowCriteriaItem{{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "false"}}}
	for _, tc := range []struct {
		name         string
		autolaunched bool
	}{{"autolaunched", true}, {"event", false}} {
		t.Run(tc.name, func(t *testing.T) {
			frame := newFlowFrame()
			frame.autolaunched = tc.autolaunched
			_, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, storage.ObjectDefinition{}, frame)
			if supported {
				t.Fatal("uncaptured ordered route admitted")
			}
		})
	}
	rule := storage.FlowRule{TriggerType: "PlatformEvent", Branches: []storage.FlowBranch{branch}}
	if _, _, err := engine.selectFlowBranch(rule, storage.Record{}, storage.ObjectDefinition{}); err == nil {
		t.Fatal("uncaptured flat event route admitted")
	}
}

func TestR6FlowUncapturedUserIDLiteral(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	for _, literal := range []string{"005000000000001", "005000000000001AAA"} {
		_, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$User.Id", Operation: "equals", Value: literal})
		if !handled || supported {
			t.Fatalf("literal %q: supported=%v handled=%v", literal, supported, handled)
		}
	}
}

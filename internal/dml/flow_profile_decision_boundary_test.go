package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowProfileDecisionUncapturedOperator(t *testing.T) {
	engine := &Engine{Org: &storage.OrgState{}}
	frame := &flowFrame{capturedAfterCreate: true, saveDecisions: true}
	branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "$Profile.Name", Operation: "notequal", Value: "{!$Record.Title}"}}}
	matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, storage.ObjectDefinition{}, frame)
	if matched || supported {
		t.Fatalf("uncaptured operator changed: matched=%v supported=%v", matched, supported)
	}
}

func BenchmarkFlowProfileDecisionUncapturedOperator(b *testing.B) {
	engine := &Engine{Org: &storage.OrgState{}}
	frame := &flowFrame{capturedAfterCreate: true, saveDecisions: true}
	branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "$Profile.Name", Operation: "notequal", Value: "{!$Record.Title}"}}}
	for i := 0; i < b.N; i++ {
		matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, storage.ObjectDefinition{}, frame)
		if matched || supported {
			b.Fatal("uncaptured operator changed")
		}
	}
}

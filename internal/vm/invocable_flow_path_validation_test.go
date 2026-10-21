package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestInvocableFlowPathTrackedReferenceValidation(t *testing.T) {
	tracked := storage.FlowStep{Kind: "reference", Target: "entry", VisitKey: "entry"}
	untracked := storage.FlowStep{Kind: "reference", Target: "entry"}
	for _, test := range []struct {
		name  string
		entry storage.FlowStep
		body  []storage.FlowStep
		want  bool
	}{
		{
			name: "tracked cycle", entry: tracked, want: true,
			body: []storage.FlowStep{
				{Kind: "assignment", Assignment: storage.FlowAssignment{Operator: "Assign"}},
				{Kind: "decision", Branches: []storage.FlowBranch{{Steps: []storage.FlowStep{tracked}}}},
			},
		},
		{name: "untracked cycle", entry: untracked, body: []storage.FlowStep{untracked}},
		{name: "untracked closing edge", entry: tracked, body: []storage.FlowStep{untracked}},
		{name: "untracked entry with tracked closing edge", entry: untracked, body: []storage.FlowStep{tracked}},
		{name: "mismatched tracked closing edge", entry: tracked, body: []storage.FlowStep{{Kind: "reference", Target: "entry", VisitKey: "different"}}},
		{name: "untracked entry with nested tracked closing edge", entry: untracked, body: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{{Steps: []storage.FlowStep{tracked}}}}}},
		{name: "missing tracked target", entry: storage.FlowStep{Kind: "reference", Target: "missing", VisitKey: "missing"}},
		{
			name: "unsupported after tracked backedge", entry: tracked,
			body: []storage.FlowStep{tracked, {Kind: "assignment", Assignment: storage.FlowAssignment{Operator: "Add"}}},
		},
		{
			name: "faulted tracked backedge", entry: tracked,
			body: []storage.FlowStep{{Kind: "reference", Target: "entry", VisitKey: "entry", FaultTarget: "recover"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := invocableFlowActionStepsSupportedWithTargets([]storage.FlowStep{test.entry}, map[string][]storage.FlowStep{"entry": test.body}); got != test.want {
				t.Fatalf("Invocable validation = %t, want %t", got, test.want)
			}
		})
	}
}

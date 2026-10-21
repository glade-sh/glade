package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The Invocable.Action route (Engine.InvocableFlow) compares Contains without
// regard to case, as the Flow condition operators reference documents. Null on
// either side is false (native rows: TestInvocableFlowActionContainsDecisionRows).
// The mixed-case rows have no native capture yet.
func TestInvocableFlowContainsIsCaseInsensitive(t *testing.T) {
	rule := storage.FlowRule{
		Name: "InvocableContains", Active: true, ProcessType: "AutoLaunchedFlow",
		Variables: []storage.FlowVariable{
			{Name: "haystack", DataType: "String", IsInput: true},
			{Name: "needle", DataType: "String", IsInput: true},
			{Name: "route", DataType: "String", IsOutput: true},
		},
		Steps: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{
			{Name: "hit", Criteria: []storage.WorkflowCriteriaItem{{Field: "haystack", Operation: "Contains", Value: "{!needle}"}},
				Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Name: "A", Target: "route", Operator: "Assign", LiteralValue: "hit"}}}},
			{Name: "miss", Default: true,
				Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Name: "B", Target: "route", Operator: "Assign", LiteralValue: "miss"}}}},
		}}},
	}
	for _, tc := range []struct{ name, haystack, needle, want string }{
		{"same case", "UnoDos", "Dos", "hit"},
		{"upper haystack", "UNODOS", "Dos", "hit"},
		{"lower haystack", "unodos", "Dos", "hit"},
		{"upper needle", "unodos", "DOS", "hit"},
		{"absent", "Uno", "Dos", "miss"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			engine := NewEngine(&org)
			engine.InvocableFlow = true
			out, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{Scalars: map[string]storage.Value{
				"haystack": storage.StringValue(tc.haystack), "needle": storage.StringValue(tc.needle),
			}})
			if err != nil {
				t.Fatal(err)
			}
			if got := out.Scalars["route"].String; got != tc.want {
				t.Fatalf("route = %q, want %q", got, tc.want)
			}
		})
	}
}

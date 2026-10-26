package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A scalar String input compared with the Contains operator, run through the
// Flow.Interview path (InvocableFlow unset). Contains is false when the left
// side is null and true on a substring match, ignoring case. Native scalar rows:
// flow_text_email/api{62,67}/org.tsv, direct_A through direct_D. The address
// strings here are local variations of those native text comparisons.
func TestAutolaunchedFlowScalarContainsDecision(t *testing.T) {
	rule := storage.FlowRule{
		Name: "ScalarContains", Active: true, ProcessType: "AutoLaunchedFlow",
		Variables: []storage.FlowVariable{
			{Name: "target", DataType: "String", IsInput: true},
			{Name: "route", DataType: "String", IsOutput: true},
		},
		Steps: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{
			{Name: "hasAt", Criteria: []storage.WorkflowCriteriaItem{{Field: "target", Operation: "Contains", Value: "Mail."}},
				Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Name: "A", Target: "route", Operator: "Assign", LiteralValue: "address"}}}},
			{Name: "other", Default: true,
				Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Name: "B", Target: "route", Operator: "Assign", LiteralValue: "id"}}}},
		}}},
	}
	for _, tc := range []struct {
		name  string
		input map[string]storage.Value
		want  string
	}{
		{"match", map[string]storage.Value{"target": storage.StringValue("x@Mail.example")}, "address"},
		{"lower case value", map[string]storage.Value{"target": storage.StringValue("x@mail.example")}, "address"},
		{"upper case value", map[string]storage.Value{"target": storage.StringValue("X@MAIL.EXAMPLE")}, "address"},
		{"no match", map[string]storage.Value{"target": storage.StringValue("003000000000001")}, "id"},
		{"empty", map[string]storage.Value{"target": storage.StringValue("")}, "id"},
		{"null", map[string]storage.Value{"target": storage.NullValue()}, "id"},
		{"omitted", map[string]storage.Value{}, "id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			engine := NewEngine(&org)
			out, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{Scalars: tc.input})
			if err != nil {
				t.Fatal(err)
			}
			if got := out.Scalars["route"].String; got != tc.want {
				t.Fatalf("route = %q, want %q", got, tc.want)
			}
		})
	}
}

package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Multi-iteration capture: internal/apextest/testdata/conformance/r6flow_multi/org.tsv:2-13.
func TestR6FlowCapturedNumericLoopBodySafety(t *testing.T) {
	for _, tc := range []struct {
		name, target, operator, literal string
		want                            bool
	}{
		{"captured", "Count", "Add", "1.0", true},
		{"undeclared", "Missing", "Add", "1.0", false},
		{"otherOperator", "Count", "Subtract", "1.0", false},
		{"notNumeric", "Count", "Add", "bad", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assign := storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "Items", Operator: "Assign", SourceField: "Records"}}
			loop := storage.FlowStep{Kind: "loop", Loop: storage.FlowLoop{CollectionReference: "Items", IterationOrder: "Asc", Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: tc.target, Operator: tc.operator, LiteralValue: tc.literal, Formula: tc.literal}}}}}
			rule := storage.FlowRule{TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Variables: []storage.FlowVariable{{Name: "Items", DataType: "SObject", ObjectType: "Account", IsCollection: true}, {Name: "Count", DataType: "Number"}}, Steps: []storage.FlowStep{assign, loop}}
			if got := flowTriggeredStepsSafe(rule, rule.Steps, "before-save"); got != tc.want {
				t.Fatalf("safe=%v want=%v", got, tc.want)
			}
		})
	}
}

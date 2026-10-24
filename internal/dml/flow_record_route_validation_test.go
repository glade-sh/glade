package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowRecordBoundaryRouteValidation(t *testing.T) {
	for _, compact := range []bool{false, true} {
		for _, test := range []struct {
			name, label, trigger, operation string
			nested, want                    bool
		}{
			{name: "before-save", label: "before-save", trigger: "RecordBeforeSave", operation: "Create"},
			{name: "before-delete-direct", label: "before-delete", trigger: "RecordBeforeDelete", operation: "Delete", want: true},
			{name: "before-delete-subflow", label: "before-delete", trigger: "RecordBeforeDelete", operation: "Delete", nested: true, want: true},
			{name: "event-direct", label: "platform-event", trigger: "PlatformEvent", want: true},
			{name: "event-subflow-rejected", label: "platform-event", trigger: "PlatformEvent", nested: true},
		} {
			t.Run(test.name+map[bool]string{false: "/inline", true: "/targets"}[compact], func(t *testing.T) {
				middle := storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "Fetched.Phone", Operator: "Assign", LiteralValue: "222"}}
				if test.nested {
					middle = storage.FlowStep{Kind: "subflow", Subflow: storage.FlowSubflow{Name: "CallChild", FlowName: "BoundaryLeaf"}}
				}
				steps := []storage.FlowStep{{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{Name: "Fetched", ObjectName: "Account", GetFirstRecordOnly: true, StoreOutputAutomatically: true}}, middle, {Kind: "recordUpdate", RecordUpdate: storage.FlowRecordUpdate{Name: "Persist", ObjectName: "Account", InputReference: "returned"}}}
				rule := storage.FlowRule{TriggerType: test.trigger, RecordTriggerType: test.operation}
				if compact {
					rule.StepTargets = map[string][]storage.FlowStep{"entry": steps}
					steps = []storage.FlowStep{{Kind: "reference", Target: "entry"}}
				}
				if got := flowTriggeredStepsSafe(rule, steps, test.label); got != test.want {
					t.Fatalf("record route accepted=%t, native=%t", got, test.want)
				}
			})
		}
	}
}

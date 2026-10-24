package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowExplicitLookupElementNullDecisionDirectAndTrigger(t *testing.T) {
	for _, route := range []string{"direct_interview", "trigger_subflow", "before_save", "after_save"} {
		for _, matched := range []bool{false, true} {
			t.Run(route+"/matched="+map[bool]string{false: "false", true: "true"}[matched], func(t *testing.T) {
				org := testOrg()
				engine := NewEngine(&org)
				if matched {
					rows := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Target")}}})
					if !rows[0].Success {
						t.Fatal(rows)
					}
				}
				frame := newFlowFrame()
				frame.variables["message"] = storage.FlowVariable{Name: "Message", DataType: "String"}
				lookup := storage.FlowRecordLookup{
					Name: "Find", ObjectName: "Account", GetFirstRecordOnly: true,
					Criteria:          []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "Target"}},
					OutputAssignments: []storage.FlowAssignment{{Target: "Message", SourceField: "Name"}},
				}
				if err := engine.applyFlowRecordLookupStep(lookup, storage.Record{}, storage.ObjectDefinition{}, frame); err != nil {
					t.Fatal(err)
				}
				branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "Find", Operation: "isNull", Value: "false"}}}
				if matches, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, storage.ObjectDefinition{}, frame); !supported || !matches {
					t.Fatalf("explicit-output element remains non-null after either lookup outcome: matched=%t matches=%t supported=%t", matched, matches, supported)
				}
			})
		}
	}
}

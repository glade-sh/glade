package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestInvocableFlowSubflowRecordRoute(t *testing.T) {
	for _, compact := range []bool{false, true} {
		t.Run(map[bool]string{false: "inline", true: "targets"}[compact], func(t *testing.T) {
			steps := []storage.FlowStep{
				{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{Name: "Fetched", ObjectName: "Account", GetFirstRecordOnly: true, StoreOutputAutomatically: true}},
				{Kind: "subflow", Subflow: storage.FlowSubflow{Name: "CallChild", FlowName: "BoundaryLeaf", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "candidate", SourceField: "Fetched"}}, OutputAssignments: []storage.FlowAssignment{{Target: "returned", SourceField: "candidate"}}}},
			}
			rule := storage.FlowRule{ProcessType: "AutoLaunchedFlow", Variables: []storage.FlowVariable{
				{Name: "sourceName", DataType: "String", IsInput: true}, {Name: "returned", DataType: "SObject", ObjectType: "Account", IsOutput: true},
			}, Steps: steps}
			if compact {
				rule.StepTargets = map[string][]storage.FlowStep{"entry": steps}
				rule.Steps = []storage.FlowStep{{Kind: "reference", Target: "entry"}}
			}
			if !invocableFlowActionSupported(rule) {
				t.Fatal("native invocable Flow accepts lookup-to-subflow record inputs and returned records")
			}
		})
	}
}

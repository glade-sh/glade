package automation

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowConstantAssignmentDirectAndRoutedLowering(t *testing.T) {
	for _, route := range []string{"direct_interview", "trigger_subflow", "before_save", "after_save"} {
		t.Run(route, func(t *testing.T) {
			raw := flowXML{
				Constants:   []flowConstantXML{{Name: "Greeting", DataType: "String", Value: flowValueXML{StringValue: "hello & goodbye"}}},
				Assignments: []flowAssignmentXML{{Name: "Copy", Items: []flowAssignmentItemXML{{AssignToReference: "Message", Operator: "Assign", Value: flowValueXML{ElementReference: "Greeting"}}}}},
			}
			if route == "before_save" || route == "after_save" {
				raw.Start = flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}
				if route == "after_save" {
					raw.Start.TriggerType = "RecordAfterSave"
				}
			}
			variables := map[string]flowVariableXML{"message": {Name: "Message", DataType: "String"}}
			branch, diagnostics := flowPopulateBranchFromTarget(storage.FlowBranch{}, "Copy", raw, nil, variables)
			if len(diagnostics) != 0 || len(branch.Steps) != 1 {
				t.Fatalf("lowering: %+v %v", branch.Steps, diagnostics)
			}
			assignment := branch.Steps[0].Assignment
			if assignment.LiteralValue != "hello & goodbye" || assignment.SourceField != "" {
				t.Fatalf("constant was not lowered to its literal: %+v", assignment)
			}
		})
	}
}

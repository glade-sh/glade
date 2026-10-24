package automation

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestFlowChainConstantRecordUpdateAndScalarOperandLoweringRoutes(t *testing.T) {
	for _, trigger := range []string{"", "RecordBeforeSave", "RecordAfterSave"} {
		t.Run("trigger="+trigger, func(t *testing.T) {
			raw := flowXML{Start: flowStartXML{Object: "Contact", TriggerType: trigger, RecordTriggerType: "CreateAndUpdate"}, Constants: []flowConstantXML{{Name: "Greeting", DataType: "String", Value: flowValueXML{StringValue: "Not Applicable"}}, {Name: "Empty", DataType: "String", Value: flowValueXML{EmptyStringValue: true}}}, RecordUpdates: []flowRecordUpdateXML{{Name: "Write", InputReference: "$Record", Fields: []flowFieldAssignmentXML{{Field: "Description", Value: flowValueXML{ElementReference: "Greeting"}}, {Field: "Title", Value: flowValueXML{ElementReference: "Empty"}}}}}}
			branch, diagnostics := flowPopulateBranchFromTarget(storage.FlowBranch{}, "Write", raw, nil, nil)
			if len(diagnostics) != 0 || len(branch.FieldUpdates) != 2 {
				t.Fatalf("updates=%+v diagnostics=%v", branch.FieldUpdates, diagnostics)
			}
			if got := branch.FieldUpdates[0]; got.LiteralValue != "Not Applicable" || got.SourceField != "" {
				t.Fatalf("constant record update: %+v", got)
			}
			if got := branch.FieldUpdates[1]; got.Formula != "NULL" || got.SourceField != "" {
				t.Fatalf("empty constant record update: %+v", got)
			}
		})
	}
}

func TestFlowChainNumberedLookupDirectBeforeAndAfterSaveLowering(t *testing.T) {
	for _, trigger := range []string{"", "RecordBeforeSave", "RecordAfterSave"} {
		t.Run("trigger="+trigger, func(t *testing.T) {
			raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: trigger, RecordTriggerType: "CreateAndUpdate"}, RecordLookups: []flowRecordLookupXML{{Name: "Find", Object: "RecordType", FilterLogic: "1 AND 2", GetFirstRecordOnly: true, StoreOutputAutomatically: true, Filters: []flowFilterXML{{Field: "SobjectType", Operator: "EqualTo", Value: flowValueXML{StringValue: "Account"}}, {Field: "DeveloperName", Operator: "EqualTo", Value: flowValueXML{StringValue: "Closed"}}}}}}
			branch, diagnostics := flowPopulateBranchFromTarget(storage.FlowBranch{}, "Find", raw, nil, nil)
			if len(diagnostics) != 0 || len(branch.Steps) != 1 {
				t.Fatalf("steps=%+v diagnostics=%v", branch.Steps, diagnostics)
			}
			got := branch.Steps[0].RecordLookup
			if got.CriteriaLogic != "and" || len(got.Criteria) != 2 {
				t.Fatalf("numbered conjunction: %+v", got)
			}
		})
	}
}

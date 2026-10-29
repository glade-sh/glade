package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Preserve the existing invocable resource reader when the left operand came
// from Get Records. Native non-invocable rows must not replace this route.
func TestFlowInvocableLookupResourceOperandsPreserveRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, operation, dataType string
			left, right               storage.Value
		}{
			{"contains-resource", "contains", "String", storage.StringValue("Preserved"), storage.StringValue("serve")},
			{"number-resource", "equals", "Number", storage.IntegerValue(23), storage.IntegerValue(23)},
			{"case-resource", "equals", "String", storage.StringValue("Preserved"), storage.StringValue("PRESERVED")},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := testOrg()
				org.APIVersion = api
				definition := org.Objects["Account"].Definition
				definition.Fields["Operand__c"] = storage.Field{APIName: "Operand__c", Type: storage.FieldString}
				if tc.dataType == "Number" {
					definition.Fields["Operand__c"] = storage.Field{APIName: "Operand__c", Type: storage.FieldInteger}
				}
				frame := newFlowFrame()
				frame.lookupOutputs["found"] = flowLookupOutput{definition: definition, record: storage.Record{Object: "Account", Fields: map[string]storage.Value{"Operand__c": tc.left}}}
				frame.variables["needle"] = storage.FlowVariable{Name: "needle", DataType: tc.dataType, IsInput: true}
				frame.scalars["needle"] = tc.right
				engine := NewEngine(&org)
				engine.InvocableFlow = true
				branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "Found.Operand__c", Operation: tc.operation, Value: "{!needle}"}}}
				matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, definition, frame)
				if !matched || !supported {
					t.Fatalf("base invocable resource route lost: matched=%v supported=%v", matched, supported)
				}
			})
		}
	}
}

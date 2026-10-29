package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Resource forms outside the captured String EqualTo shape retain the release
// dispatcher. These assertions preserve existing behavior, not native parity.
func TestFlowLookupResourceCriteriaPreserveFallback(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, operation, value, source string
			left, right                    storage.Value
			want                           bool
		}{
			{"null-isnull-source", "isnull", "", "needle", storage.NullValue(), storage.StringValue("unused"), true},
			{"text-isnull-source", "isnull", "", "needle", storage.StringValue("preserved"), storage.StringValue("unused"), false},
			{"number-isnull-source", "isnull", "", "needle", storage.IntegerValue(23), storage.IntegerValue(23), false},
			{"number-equals-token", "equals", "{!needle}", "", storage.IntegerValue(23), storage.IntegerValue(23), false},
			{"null-equals-token", "equals", "{!needle}", "", storage.NullValue(), storage.StringValue("unused"), false},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := testOrg()
				org.APIVersion = api
				definition := org.Objects["Account"].Definition
				definition.Fields["Operand__c"] = storage.Field{APIName: "Operand__c", Type: storage.FieldString}
				dataType := "String"
				if tc.left.Kind == storage.ValueInteger {
					definition.Fields["Operand__c"] = storage.Field{APIName: "Operand__c", Type: storage.FieldInteger}
					dataType = "Number"
				}
				frame := newFlowFrame()
				frame.lookupOutputs["found"] = flowLookupOutput{definition: definition, record: storage.Record{Object: "Account", Fields: map[string]storage.Value{"Operand__c": tc.left}}}
				frame.variables["needle"] = storage.FlowVariable{Name: "needle", DataType: dataType}
				frame.scalars["needle"] = tc.right
				branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "Found.Operand__c", Operation: tc.operation, Value: tc.value, SourceField: tc.source}}}
				engine := NewEngine(&org)
				matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, definition, frame)
				if matched != tc.want || !supported {
					t.Fatalf("release lookup fallback changed: matched=%v supported=%v; want matched=%v supported=true", matched, supported, tc.want)
				}
			})
		}
	}
}

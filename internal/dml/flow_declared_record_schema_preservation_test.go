package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// An absent record reads null only after its object and field resolve, as on
// the release base. This does not add a native unknown-field parity claim.
func TestFlowAbsentDeclaredRecordPreservesSchemaValidation(t *testing.T) {
	for _, tc := range []struct {
		name, object, field string
		want                bool
	}{
		{"known-field", "Account", "Name", true},
		{"unknown-field", "Account", "Missing__c", false},
		{"unknown-object", "Absent__c", "Name", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			frame := newFlowFrame()
			frame.variables["record"] = storage.FlowVariable{Name: "record", DataType: "SObject", ObjectType: tc.object}
			value, handled := engine.flowSObjectVariableFieldValue(storage.WorkflowCriteriaItem{Field: "record." + tc.field, Operation: "isnull", Value: "true"}, frame)
			if handled != tc.want || handled && value.Kind != storage.ValueNull {
				t.Fatalf("base schema resolution changed: handled=%v value=%v; want handled=%v", handled, value, tc.want)
			}
		})
	}
}

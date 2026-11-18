package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The afteraccept rows exercise changed=true and changed=false entry filters on
// two records whose prior values differ. A false operand selects unchanged rows.
func TestFlowAfterSaveIsChangedBooleanOperand(t *testing.T) {
	def := storage.ObjectDefinition{Fields: map[string]storage.Field{"Choice__c": {APIName: "Choice__c", Type: storage.FieldString}}}
	rule := storage.FlowRule{TriggerType: "RecordAfterSave", RecordTriggerType: "CreateAndUpdate"}
	prior := storage.Record{Fields: map[string]storage.Value{"Choice__c": storage.StringValue("A")}}
	for _, tc := range []struct {
		name, current, operand string
		want                   bool
	}{
		{"changed true", "B", "true", true},
		{"changed false", "B", "false", false},
		{"unchanged true", "A", "true", false},
		{"unchanged false", "A", "false", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := storage.Record{Fields: map[string]storage.Value{"Choice__c": storage.StringValue(tc.current)}}
			got, ok := evaluateCapturedAfterSaveChangedCriterion(storage.WorkflowCriteriaItem{Field: "Choice__c", Operation: "isChanged", Value: tc.operand}, now, def, "", &prior, rule, false)
			if !ok || got != tc.want {
				t.Fatalf("got %v, supported %v; want %v", got, ok, tc.want)
			}
		})
	}
}

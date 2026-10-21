package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// TestFlowStartIsNullHonoursBooleanOperand pins the IsNull start-filter rows
// captured on a scratch org at API 62.0, 67.0 and 68.0. A record-triggered
// before-save flow filtered on "P4_Date__c IsNull false" ran only for the Case
// inserted with the date set (Subject became "flow-set"). The Case inserted
// without the date kept Subject "orig". "IsNull true" is the inverse.
func TestFlowStartIsNullHonoursBooleanOperand(t *testing.T) {
	definition := storage.ObjectDefinition{
		APIName: "Case",
		Fields: map[string]storage.Field{
			"P4_Date__c": {APIName: "P4_Date__c", Type: storage.FieldDate},
		},
	}
	withDate := storage.Record{Object: "Case", Fields: map[string]storage.Value{"P4_Date__c": storage.StringValue("2026-01-02")}}
	withoutDate := storage.Record{Object: "Case", Fields: map[string]storage.Value{}}
	cases := []struct {
		name   string
		value  string
		record storage.Record
		want   bool
	}{
		{"IsNull false, date set (native: flow ran)", "false", withDate, true},
		{"IsNull false, date absent (native: flow skipped)", "false", withoutDate, false},
		{"IsNull true, date set", "true", withDate, false},
		{"IsNull true, date absent", "true", withoutDate, true},
		{"IsNull without operand keeps true semantics, date absent", "", withoutDate, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := storage.WorkflowCriteriaItem{Field: "P4_Date__c", Operation: "isnull", Value: tc.value}
			got, ok := evaluateWorkflowCriteria(item, tc.record, definition, "")
			if !ok || got != tc.want {
				t.Fatalf("evaluateWorkflowCriteria(%+v) = %v, %v; want %v, true", item, got, ok, tc.want)
			}
		})
	}
}

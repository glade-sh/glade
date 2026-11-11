package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native heterogeneous after-save rows assign DATEVALUE(CreatedDate)
// after an update; CreatedDate comes from the stored record's system fields.
func TestFlowAfterSaveDateFormulaReadsCreatedDate(t *testing.T) {
	def := storage.ObjectDefinition{APIName: "Fern__c", Fields: map[string]storage.Field{"CreatedDate": {APIName: "CreatedDate", Type: storage.FieldDateTime, DisplayType: "DATETIME"}}}
	record := storage.Record{Object: "Fern__c", System: storage.SystemFields{CreatedDate: "2026-01-02T03:04:05.000Z"}}
	field := storage.Field{APIName: "Day__c", Type: storage.FieldDate, DisplayType: "DATE"}
	value, isNull, ok := EvaluateRecordFormulaValueInOrgWithOptions("DATEVALUE(CreatedDate)", field, &storage.OrgState{}, def, record, FormulaEvaluationOptions{FlowContext: true, capturedAfterSaveSystemFields: true})
	if !ok || isNull || value.Kind != storage.ValueDate || value.String != "2026-01-02" {
		t.Fatalf("got %#v, null=%v supported=%v", value, isNull, ok)
	}
	value, isNull, ok = EvaluateRecordFormulaValueInOrgWithOptions("DATEVALUE(CreatedDate)", field, &storage.OrgState{}, def, record, FormulaEvaluationOptions{FlowContext: true})
	if !ok || !isNull {
		t.Fatalf("uncaptured flow surface changed: %#v, null=%v supported=%v", value, isNull, ok)
	}

}

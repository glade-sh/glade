package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Native anonymous and @IsTest observations at API 62/67/68 keep the insert
// caller's COUNT null; a fresh query observes the child count. Update, upsert
// and SUM are existing noninterference controls, not additional native rows.
func TestInsertedSummaryCountPreservesCallerProjection(t *testing.T) {
	for _, operation := range []string{"insert", "update", "upsert"} {
		t.Run(operation, func(t *testing.T) {
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["LineCount__c"] = storage.Field{APIName: "LineCount__c", Type: storage.FieldSummary, DisplayType: "INTEGER", SummaryOperation: "count"}
			account.Definition.Fields["LineSum__c"] = storage.Field{APIName: "LineSum__c", Type: storage.FieldSummary, DisplayType: "CURRENCY", SummaryOperation: "sum"}
			account.Records["001000000000001"] = storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Count seed")}}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			row := Object("Account")
			result := []dml.Result{{ID: "001000000000001", Success: true}}
			machine.populateDMLResultFields(&row, result, operation)
			if _, _, present := objectFieldValue(row, "LineCount__c"); present {
				t.Fatalf("%s unexpectedly refreshed caller COUNT", operation)
			}
			if _, _, present := objectFieldValue(row, "LineSum__c"); present {
				t.Fatal("caller SUM unexpectedly materialized")
			}
		})
	}
}

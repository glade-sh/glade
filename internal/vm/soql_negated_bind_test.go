package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows (scratch org, API 62.0, 67.0, 68.0): with one active and one
// inactive record, `Active__c = :!flag` and `Active__c = :(!flag)` returned
// the inactive record when flag was true, and `:-n` bound the negated number.
func TestSOQLNegatedBooleanBind(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Widget__c",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"Active__c": {APIName: "Active__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001AAA": {ID: "a00000000000001AAA", Object: "Widget__c", Fields: map[string]storage.Value{"Active__c": storage.BooleanValue(true)}},
			"a00000000000002AAA": {ID: "a00000000000002AAA", Object: "Widget__c", Fields: map[string]storage.Value{"Active__c": storage.BooleanValue(false)}},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.Globals["flag"] = Bool(true)
	for _, where := range []string{":!flag", ":(!flag)", ":! flag"} {
		got, err := machine.executeSOQL("SELECT Id FROM Widget__c WHERE Active__c = "+where, &Result{})
		if err != nil {
			t.Fatalf("%s: %v", where, err)
		}
		if got.Kind != ValueList || len(got.List) != 1 {
			t.Fatalf("%s: rows = %#v, want the single inactive record", where, got)
		}
	}
}

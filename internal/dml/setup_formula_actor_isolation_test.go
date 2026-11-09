package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"reflect"
	"testing"
)

// Supplying only the setup-resolution actor must not alter other formula
// globals; this is a local scope control, not a Salesforce parity assertion.
func TestSetupFormulaActorDoesNotChangeOtherGlobals(t *testing.T) {
	org := storage.NewOrgState()
	actor := storage.ID("005000000000002")
	primary := storage.ID(storage.DefaultUserID)
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		primary: {ID: primary, Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}},
		actor:   {ID: actor, Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002")}},
	}}
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"00e000000000001": {Fields: map[string]storage.Value{"Name": storage.StringValue("Primary")}},
		"00e000000000002": {Fields: map[string]storage.Value{"Name": storage.StringValue("Alternate")}},
	}}
	field := storage.Field{Type: storage.FieldCalculated, DisplayType: "STRING"}
	for _, formula := range []string{"$User.Id", "$Profile.Name"} {
		baseline, _, ok := EvaluateRecordFormulaValueInOrg(formula, field, &org, storage.ObjectDefinition{}, storage.Record{})
		if !ok {
			t.Fatalf("baseline %s unsupported", formula)
		}
		got, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(formula, field, &org, storage.ObjectDefinition{}, storage.Record{}, FormulaEvaluationOptions{SetupUserID: actor})
		if !ok || !reflect.DeepEqual(got, baseline) {
			t.Fatalf("%s baseline=%#v got=%#v supported=%v", formula, baseline, got, ok)
		}
	}
}

package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Custom currency and date outcomes at APIs 62 and 67: boundary-fr-org.tsv:2-6.
func TestJSONFormatFrenchCustomFieldTypes(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Synthetic__c": {Definition: storage.ObjectDefinition{APIName: "Synthetic__c", Fields: map[string]storage.Field{
			"Money__c": {APIName: "Money__c", DisplayType: "CURRENCY", Scale: 2},
			"Day__c":   {APIName: "Day__c", DisplayType: "DATE"},
		}}},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("fr_FR")}})
	for _, tc := range []struct{ field, before, want string }{{"Money__c", "1234.5", "$1,234.50"}, {"Money__c", "-12.34", "-$12.34"}, {"Day__c", "1/2/2030", "02/01/2030"}, {"Day__c", "6/15/2028", "15/06/2028"}} {
		value := Object("Synthetic__c")
		value.Fields["formatted"] = String(tc.before)
		machine.applyJSONQueryFunctions(&value, []string{"FORMAT(" + tc.field + ") formatted"})
		if got := value.Fields["formatted"].Text; got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, got, tc.want)
		}
	}
}

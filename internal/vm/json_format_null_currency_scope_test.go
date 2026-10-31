package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// USD single-currency behavior is captured at sobject_json3/org.tsv:28-29.
// Other currencies and multicurrency preserve the pre-change behavior; they are
// not claims about Salesforce behavior without a native capture.
func TestJSONFormatNullCurrencyScope(t *testing.T) {
	for _, tc := range []struct {
		name, currency string
		multi, covered bool
	}{
		{"usd-single", "USD", false, true},
		{"eur-single", "EUR", false, false},
		{"usd-multi", "USD", true, false},
		{"eur-multi", "EUR", true, false},
	} {
		for _, present := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/missing", true: "/null"}[present], func(t *testing.T) {
				org := storage.OrgState{Features: []string{}, Objects: map[string]storage.ObjectState{"Synthetic__c": {Definition: storage.ObjectDefinition{APIName: "Synthetic__c", Fields: map[string]storage.Field{"Money__c": {APIName: "Money__c", DisplayType: "CURRENCY", Scale: 2}}}}}}
				if tc.multi {
					org.Features = []string{"MultiCurrency"}
				}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("fr_FR"), "DefaultCurrencyIsoCode": storage.StringValue(tc.currency)}})
				value := Object("Synthetic__c")
				if present {
					value.Fields["formatted"] = Null
				}
				machine.applyJSONQueryFunctions(&value, []string{"FORMAT(Money__c) formatted"})
				got, found := value.Fields["formatted"]
				if tc.covered {
					if !found || got.Kind != ValueString || got.Text != "" {
						t.Fatalf("got %#v present=%t; want empty string", got, found)
					}
					return
				}
				if found != present || (found && got.Kind != ValueNull) {
					t.Fatalf("got %#v present=%t; want original presence=%t and null", got, found, present)
				}
			})
		}
	}
}

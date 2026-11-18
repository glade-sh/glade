package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A queried User carries the explicit-field marker, so resolving its Id walks
// every field with a lower-cased alias check. Field defaults run for every
// field of every DML row; only the currency default needs the running user.
func TestDefaultValueForRecordFieldReadsUserOnlyForCurrency(t *testing.T) {
	user := Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{
		"Id":                     String("005000000000001"),
		"Name":                   String("Running User"),
		"Username":               String("running@example.com"),
		"Email":                  String("running@example.com"),
		"Alias":                  String("ruser"),
		"TimeZoneSidKey":         String("UTC"),
		"LocaleSidKey":           String("en_US"),
		"EmailEncodingKey":       String("UTF-8"),
		"LanguageLocaleKey":      String("en_US"),
		"ProfileId":              String("00e000000000001"),
		"DefaultCurrencyIsoCode": String("EUR"),
	}}
	for name := range user.Fields {
		markExplicitSObjectField(&user, name)
	}
	accountDefinition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"Name":            {APIName: "Name", Type: storage.FieldString},
		"Detail__c":       {APIName: "Detail__c", Type: storage.FieldString},
		"Mirror__pc":      {APIName: "Mirror__pc", Type: storage.FieldString},
		"CurrencyIsoCode": {APIName: "CurrencyIsoCode", Type: storage.FieldString},
	}}
	contactDefinition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
		"Mirror__c": {APIName: "Mirror__c", Type: storage.FieldString},
	}}
	org := &storage.OrgState{Objects: map[string]storage.ObjectState{
		"Account": {Definition: accountDefinition, Records: map[storage.ID]storage.Record{}},
		"Contact": {Definition: contactDefinition, Records: map[storage.ID]storage.Record{}},
		"User": {Definition: storage.ObjectDefinition{APIName: "User"}, Records: map[storage.ID]storage.Record{
			"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"DefaultCurrencyIsoCode": storage.StringValue("EUR")}},
		}},
	}}
	org.Features = []string{"MultiCurrency"}
	machine := &VM{Org: org, executionUser: user}
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}}

	if value, ok := machine.defaultValueForRecordField(accountDefinition, record, accountDefinition.Fields["CurrencyIsoCode"]); !ok || value.String != "EUR" {
		t.Fatalf("currency default = %#v, %v; want the running user's EUR", value, ok)
	}
	if _, ok := machine.defaultValueForRecordField(accountDefinition, record, accountDefinition.Fields["Mirror__pc"]); ok {
		t.Fatal("business account acquired a person-only mirror default")
	}
	for _, name := range []string{"Detail__c", "Name"} {
		field := accountDefinition.Fields[name]
		allocations := testing.AllocsPerRun(50, func() {
			if _, ok := machine.defaultValueForRecordField(accountDefinition, record, field); ok {
				t.Fatalf("%s without a default produced a value", name)
			}
		})
		if allocations != 0 {
			t.Fatalf("%s without a default allocated %.0f times; the running user and lower-cased names belong only to the currency and person-account paths", name, allocations)
		}
	}
}

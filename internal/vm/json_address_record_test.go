package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// An ordinary SObject record typed Address (the standard Address object)
// keeps the SObject JSON shape: attributes plus its own fields. Only a real
// System.Address value uses the compound property list (custom_address_fields
// followup rows J01-J11).
func TestJSONAddressRecordKeepsSObjectShape(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, suppress := range []bool{false, true} {
			record := Object("Address")
			record.Fields["Name"] = String("Owned record")
			record.Fields["Street"] = String("1 Main St")
			record.Fields["City"] = Null
			org := testDataOrg()
			org.APIVersion = api
			machine := New(nil)
			machine.SetOrg(&org)
			for _, encoded := range []any{machine.jsonFromValueForSerialize(record, suppress), jsonFromValue(record, suppress)} {
				for _, marshal := range []func(any) ([]byte, error){
					jsonMarshalNoEscape,
					func(v any) ([]byte, error) { return jsonMarshalNoEscapeIndent(v, "", "  ") },
				} {
					raw, err := marshal(encoded)
					if err != nil {
						t.Fatal(err)
					}
					var got map[string]any
					if err := json.Unmarshal(raw, &got); err != nil {
						t.Fatal(err)
					}
					attrs, ok := got["attributes"].(map[string]any)
					if !ok || attrs["type"] != "Address" || got["Name"] != "Owned record" || got["Street"] != "1 Main St" {
						t.Fatalf("api %s suppress %v: record fields lost: %s", api, suppress, raw)
					}
					if _, compound := got["street"]; compound {
						t.Fatalf("api %s suppress %v: record serialized as System.Address: %s", api, suppress, raw)
					}
				}
			}
		}
	}
}

// A queried System.Address still serializes as the compound value under both
// serialize and serializePretty overloads (followup rows J01/J05 shape).
func TestJSONAddressSystemValueKeepsCompoundShape(t *testing.T) {
	const body = `Account a=new Account(Name='Json Address',BillingStreet='1 Main St',BillingCity='Springfield'); insert a;
System.Address b=[SELECT BillingAddress FROM Account WHERE Id=:a.Id].BillingAddress;
String want='{"city":"Springfield","country":null,"geocodeAccuracy":null,"latitude":null,"longitude":null,"postalCode":null,"state":null,"street":"1 Main St"}';
System.assertEquals(want, JSON.serialize(b));
System.assertEquals(want, JSON.serialize(b, true));
for (String p : new List<String>{JSON.serializePretty(b), JSON.serializePretty(b, true)}) {
	System.assert(!p.contains('attributes'), p);
	System.assert(p.contains('"street" : "1 Main St"'), p);
	System.assert(p.contains('"country" : null'), p);
}
Map<String, Object> wrapped=(Map<String, Object>)JSON.deserializeUntyped(JSON.serialize(new Map<String, Object>{'k'=>b}));
System.assertEquals('1 Main St', ((Map<String, Object>)wrapped.get('k')).get('street'));`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Account")
			org.APIVersion = api
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

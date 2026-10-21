package vm

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The declared Address methods are documented. Owned native observations
// independently cover the dynamic cast accessors; direct expressions here are
// local dispatch regressions, not additional native captures.
// Evidence: Apex Reference Guide, Address Class, and the owned
// schema_constructor_population_native.json roundTrip.street/postalCode cells.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_reference_guide.pdf
func TestJSONCompoundAddressDirectAccessors(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0", "68.0"} {
		for _, receiver := range []string{"row", "rows[0]", "rows.get(0)"} {
			t.Run(api+"/"+receiver, func(t *testing.T) {
				runJSONCompoundAddressAccessors(t, api, jsonCompoundAddressAssembly+fmt.Sprintf(`
System.assertEquals('12 Lake Road', %s.BillingAddress.getStreet());
System.assertEquals('99653', %s.BillingAddress.getPostalCode());
System.assertEquals(null, %s.BillingAddress.getCity());
`, receiver, receiver, receiver))
			})
		}
	}
}

func TestJSONCompoundAddressPreservesOtherValues(t *testing.T) {
	addressField := storage.Field{APIName: "BillingAddress", Type: storage.FieldAddress}
	plainMap := Map()
	plainMap.Map[mapKey(String("street"))] = String("12 Lake Road")
	plainMap.MapKeys[mapKey(String("street"))] = String("street")
	plainMap.MapOrder = []string{mapKey(String("street"))}
	addressObject := Object("System.Address")
	addressObject.Fields["street"] = String("12 Lake Road")
	for _, tc := range []struct {
		name  string
		field storage.Field
		value Value
	}{
		{"null", addressField, Null},
		{"string", addressField, String("unchanged")},
		{"number", addressField, Int(7)},
		{"boolean", addressField, Bool(false)},
		{"alreadyAddress", addressField, addressObject},
		{"otherFieldMap", storage.Field{APIName: "Description", Type: storage.FieldString}, plainMap},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// These inputs are preservation controls, including malformed stored
			// scalars. Their native acceptance is not asserted by this local check.
			got := coerceReadSObjectFieldRuntimeValue(Object("Account"), tc.value, tc.field)
			if !reflect.DeepEqual(got, tc.value) {
				t.Fatalf("read projection changed unrelated input: got=%#v want=%#v", got, tc.value)
			}
		})
	}
	for _, provenance := range []string{"classInstance", "projectClass"} {
		t.Run(provenance, func(t *testing.T) {
			receiver := Object("Account")
			receiver.classInstance = provenance == "classInstance"
			receiver.projectClass = provenance == "projectClass"
			got := coerceReadSObjectFieldRuntimeValue(receiver, plainMap, addressField)
			if !reflect.DeepEqual(got, plainMap) {
				t.Fatal("schema metadata changed a same-name user class field")
			}
		})
	}
}

func TestJSONCompoundAddressAccessorControls(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			runJSONCompoundAddressAccessors(t, api, jsonCompoundAddressAssembly+`
Address dynamicAddress = (Address)row.get('BillingAddress');
System.assertEquals('12 Lake Road', dynamicAddress.getStreet());
System.assertEquals('99653', dynamicAddress.getPostalCode());
System.assertEquals(null, dynamicAddress.getCity());
Account ordinary = new Account(BillingStreet='18 Pine Road', BillingPostalCode='90210', BillingCity='Harbor');
System.assertEquals('18 Pine Road', ordinary.BillingAddress.getStreet());
System.assertEquals('90210', ordinary.BillingAddress.getPostalCode());
System.assertEquals('Harbor', ordinary.BillingAddress.getCity());
System.assertEquals('HARBOR', ordinary.BillingAddress.getCity().toUpperCase());
System.assertEquals('HARBOR', row.Name.toUpperCase());
Account empty = new Account();
System.assertEquals(null, empty.get('BillingAddress'));
`)
		})
	}
}

func TestJSONCompoundAddressPreservesReferenceComponents(t *testing.T) {
	for _, component := range []Value{Object("Account"), Map(), List(), Set(), Bool(true)} {
		payload := Map()
		key := mapKey(String("street"))
		payload.MapKeys[key] = String("street")
		payload.Map[key] = component
		got := coerceReadSObjectFieldRuntimeValue(Object("Account"), payload, storage.Field{Type: storage.FieldAddress})
		if got.Kind != ValueMap || got.Ref != payload.Ref || !reflect.DeepEqual(got, payload) {
			t.Fatal("unobserved compound component changed its reference carrier")
		}
	}
	payload := Map()
	key := mapKey(String("street"))
	payload.MapKeys[key] = String("street")
	payload.Map[key] = payload
	got := coerceReadSObjectFieldRuntimeValue(Object("Account"), payload, storage.Field{Type: storage.FieldAddress})
	if got.Kind != ValueMap || got.Ref != payload.Ref {
		t.Fatal("cyclic compound payload changed its reference carrier")
	}
}

const jsonCompoundAddressAssembly = `
Id recordId = '001000000000001AAA';
SObject seed = Account.SObjectType.newSObject(recordId);
Map<String,Object> fields = new Map<String,Object>(seed.getPopulatedFieldsAsMap());
fields.put('Name', 'Harbor');
fields.put('BillingAddress', new Map<String,Object>{'street'=>'12 Lake Road', 'postalCode'=>'99653'});
fields.put('CreatedDate', Datetime.newInstanceGMT(2020, 2, 3, 4, 5, 6));
Account row = (Account)JSON.deserialize(JSON.serialize(fields), Account.class);
List<Account> rows = new List<Account>{row};
System.assertEquals(recordId, row.Id);
System.assertEquals(new Set<String>{'Id', 'Name', 'BillingAddress', 'CreatedDate'}, row.getPopulatedFieldsAsMap().keySet());
`

func runJSONCompoundAddressAccessors(t *testing.T, api, source string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.APIVersion = api
	storage.EnsureStandardObject(&org, "Account")
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

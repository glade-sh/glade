package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native observations cover API 62, 67 and 68 in anonymous and named-test contexts.
func TestSchemaConstructorPopulatedID(t *testing.T) {
	for _, object := range []struct{ name, id string }{
		{"Account", "001000000000001AAA"},
		{"Contact", "003000000000001AAA"},
	} {
		t.Run(object.name, func(t *testing.T) {
			runSchemaConstructorPopulated(t, fmt.Sprintf(`
Id recordId = '%s';
SObject record = %s.SObjectType.newSObject(recordId);
Map<String,Object> populated = record.getPopulatedFieldsAsMap();
System.assertEquals(new Set<String>{'Id'}, populated.keySet());
System.assertEquals(recordId, populated.get('Id'));
`, object.id, object.name))
		})
	}
}

func TestSchemaConstructorPopulatedIDSurvivesJSONAssembly(t *testing.T) {
	runSchemaConstructorPopulated(t, `
Id recordId = '001000000000001AAA';
SObject seed = Account.SObjectType.newSObject(recordId);
Map<String,Object> fields = new Map<String,Object>(seed.getPopulatedFieldsAsMap());
fields.put('Name', 'Harbor');
fields.put('BillingAddress', new Map<String,Object>{'street'=>'12 Lake Road', 'postalCode'=>'99653'});
fields.put('CreatedDate', Datetime.newInstanceGMT(2020, 2, 3, 4, 5, 6));
Account record = (Account)JSON.deserialize(JSON.serialize(fields), Account.class);
Map<String,Object> populated = record.getPopulatedFieldsAsMap();
System.assertEquals(new Set<String>{'Id', 'Name', 'BillingAddress', 'CreatedDate'}, populated.keySet());
System.assertEquals(recordId, record.Id);
System.assertEquals('Harbor', record.Name);
System.assertEquals('12 Lake Road', ((Address)record.get('BillingAddress')).getStreet());
System.assertEquals('99653', ((Address)record.get('BillingAddress')).getPostalCode());
System.assertEquals(Datetime.newInstanceGMT(2020, 2, 3, 4, 5, 6), record.CreatedDate);
System.assertEquals(new Set<String>{'Id'}, seed.getPopulatedFieldsAsMap().keySet());
`)
}

func TestSchemaConstructorPopulatedIDControls(t *testing.T) {
	runSchemaConstructorPopulated(t, `
Id recordId = '001000000000001AAA';
System.assertEquals(false, Account.SObjectType.newSObject().getPopulatedFieldsAsMap().containsKey('Id'));
System.assertEquals(false, Account.SObjectType.newSObject(null).getPopulatedFieldsAsMap().containsKey('Id'));
System.assertEquals(false, Account.SObjectType.newSObject(null, false).getPopulatedFieldsAsMap().containsKey('Id'));
Account constructed = new Account(Id=recordId);
System.assertEquals(recordId, constructed.getPopulatedFieldsAsMap().get('Id'));
Account assigned = new Account();
assigned.Id = recordId;
System.assertEquals(recordId, assigned.getPopulatedFieldsAsMap().get('Id'));
Id recordTypeId = '012000000000000AAA';
SObject typed = Account.SObjectType.newSObject(recordTypeId, false);
System.assertEquals(null, typed.get('Id'));
System.assertEquals(false, typed.getPopulatedFieldsAsMap().containsKey('Id'));
`)
}

func runSchemaConstructorPopulated(t *testing.T, source string) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

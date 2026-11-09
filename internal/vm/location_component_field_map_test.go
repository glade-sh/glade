package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A custom Geolocation field's latitude and longitude components appear in the
// describe field map, so field-path validators that check names against it
// accept them.
func TestDescribeFieldMapIncludesLocationComponents(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
System.assert(fields.containsKey('PrimaryLocation__c'));
System.assert(fields.containsKey('PrimaryLocation__Latitude__s'));
System.assert(fields.containsKey('primarylocation__longitude__s'));
Schema.DescribeFieldResult latitude = fields.get('PrimaryLocation__Latitude__s').getDescribe();
System.assertEquals('PrimaryLocation__Latitude__s', latitude.getName());
System.assertEquals('PrimaryLocation__c', latitude.getCompoundFieldName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["PrimaryLocation__c"] = storage.Field{
		APIName:     "PrimaryLocation__c",
		Label:       "Primary Location",
		Type:        storage.FieldLocation,
		DisplayType: "LOCATION",
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

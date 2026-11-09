package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native observations: location_describe/org.tsv, API 62.0 and 67.0.
func TestLocationDescribeNative(t *testing.T) {
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Probe_Point__c"] = storage.Field{APIName: "Probe_Point__c", Type: storage.FieldLocation, DisplayType: "LOCATION"}
	account.Definition.Fields["Probe_Text__c"] = storage.Field{APIName: "Probe_Text__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
System.assertEquals(true, fields.containsKey('Probe_Point__c'));
System.assertEquals(true, fields.containsKey('Probe_Point__Latitude__s'));
System.assertEquals(true, fields.containsKey('probe_point__longitude__s'));
System.assertEquals('Probe_Point__Latitude__s', fields.get('Probe_Point__Latitude__s').getDescribe().getName());
System.assertEquals('Probe_Point__c', fields.get('Probe_Point__Latitude__s').getDescribe().getCompoundFieldName());
System.assertEquals('Probe_Point__c', fields.get('probe_point__longitude__s').getDescribe().getCompoundFieldName());
System.assertEquals(true, fields.containsKey('Probe_Text__c'));
System.assertEquals(false, fields.containsKey('Probe_Text__Latitude__s'));
System.assertEquals(false, fields.containsKey('Absent__Latitude__s'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

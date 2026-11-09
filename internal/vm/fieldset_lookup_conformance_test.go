package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecFieldSetDescribeNamespacedLookup(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.APIVersion = api
			org.Namespace = "samplepkg"
			for _, name := range []string{"CodexC163Capture", "CodexC163Second"} {
				org.Metadata.FieldSets = append(org.Metadata.FieldSets, storage.FieldSetMetadata{ObjectName: "Account", Name: name})
			}
			program, err := CompileAnonymousWithOptions(`
Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
System.assertEquals(2, sets.size());
System.assertEquals(2, sets.values().size());
System.assertEquals(2, sets.keySet().size());
System.assert(sets.containsKey('samplepkg__CodexC163Capture'));
System.assertNotEquals(null, sets.get('samplepkg__CodexC163Capture'));
System.assert(sets.containsKey('SAMPLEPKG__CODEXC163CAPTURE'));
System.assertNotEquals(null, sets.get('SAMPLEPKG__CODEXC163CAPTURE'));
System.assertEquals(false, sets.containsKey('CodexC163Capture'));
System.assertEquals(null, sets.get('CodexC163Capture'));
System.assertEquals(false, sets.containsKey('samplepkg__MissingCapture'));
System.assertEquals(null, sets.get('samplepkg__MissingCapture'));`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecFieldSetDescribeNamespacedCustomObjectLookup(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.APIVersion = api
			org.Namespace = "samplepkg"
			org.Objects["samplepkg__CodexC163Object__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "samplepkg__CodexC163Object__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
				Records:    map[storage.ID]storage.Record{},
			}
			org.Metadata.FieldSets = []storage.FieldSetMetadata{{ObjectName: "CodexC163Object__c", Name: "CodexC163Capture"}}
			program, err := CompileAnonymousWithOptions(`
Map<String, Schema.FieldSet> sets = Schema.getGlobalDescribe().get('samplepkg__CodexC163Object__c').getDescribe().fieldSets.getMap();
System.assertEquals(1, sets.keySet().size());
System.assert(sets.keySet().contains('samplepkg__codexc163capture'));
System.assert(sets.containsKey('samplepkg__CodexC163Capture'));
System.assertNotEquals(null, sets.get('samplepkg__CodexC163Capture'));
System.assert(sets.containsKey('SAMPLEPKG__CODEXC163CAPTURE'));
System.assertNotEquals(null, sets.get('SAMPLEPKG__CODEXC163CAPTURE'));
System.assertEquals(false, sets.containsKey('CodexC163Capture'));
System.assertEquals(null, sets.get('CodexC163Capture'));
System.assertEquals(false, sets.containsKey('samplepkg__MissingCapture'));
System.assertEquals(null, sets.get('samplepkg__MissingCapture'));`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

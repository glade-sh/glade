package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Describe map keys are lowercase, like fields.getMap().keySet(), and a
// namespaced field set's key carries its namespace prefix.
func TestFieldSetMapKeySetIsLowercaseAndNamespaceQualified(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.FieldSet> sets = Thing__c.SObjectType.getDescribe().fieldSets.getMap();
System.assertEquals(new Set<String>{'pkg__details'}, sets.keySet());
System.assertEquals(1, sets.size());
System.assertEquals(1, sets.values().size());
System.assertNotEquals(null, sets.get('pkg__Details'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Thing__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Thing__c",
			Fields: map[string]storage.Field{
				"pkg__Name__c": {APIName: "pkg__Name__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "Thing__c",
		Namespace:  "pkg",
		Name:       "Details",
		Fields:     []storage.FieldSetMemberMetadata{{Field: "Name__c"}},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

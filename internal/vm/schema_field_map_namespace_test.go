package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Local namespace-composition control; SF138 proves the separate API53
// declared/undeclared key matrix, not managed-package composition.
func TestSchemaFieldMapRejectsDoubleNamespacePreservingDeclaredAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Schema.SObjectField> fields=Contact.SObjectType.getDescribe().fields.getMap();
System.assert(fields.containsKey('npo02__Household__c'));
System.assert(fields.containsKey('NPO02__HOUSEHOLD__C'));
System.assertEquals(Contact.npo02__Household__c,fields.get('npo02__Household__c'));
System.assert(fields.containsKey('LocalLookup__c'));
System.assert(fields.containsKey('pkgx__LocalLookup__c'));
System.assertEquals(fields.get('LocalLookup__c'),fields.get('pkgx__LocalLookup__c'));
System.assertEquals(false,fields.containsKey('pkgx__npo02__Household__c'));
System.assertEquals(null,fields.get('pkgx__npo02__Household__c'));
System.assertEquals(false,fields.containsKey('other__LocalLookup__c'));
System.assertEquals(null,fields.get('other__LocalLookup__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Namespace: "pkgx", Objects: map[string]storage.ObjectState{"Contact": {Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
		"Id":                  {APIName: "Id", Type: storage.FieldID},
		"npo02__Household__c": {APIName: "npo02__Household__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		"LocalLookup__c":      {APIName: "LocalLookup__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
	}}}}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

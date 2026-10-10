package vm

import "testing"

// This is an implementation guard for generic schema provenance, not a new
// Salesforce claim about custom-setting lifecycle behavior.
func TestExecJSONSObjectDefaultProvenancePreservesExplicitValues(t *testing.T) {
	program, err := CompileAnonymous(`
Hierarchy_Setting__c settings = new Hierarchy_Setting__c();
System.assertEquals(false, settings.Defaulted__c);
Map<String,Object> implicitFields = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(settings));
System.assertEquals(false, implicitFields.containsKey('Defaulted__c'));

settings.Defaulted__c = false;
Map<String,Object> assignedFields = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(settings));
System.assertEquals(true, assignedFields.containsKey('Defaulted__c'));
System.assertEquals(false, assignedFields.get('Defaulted__c'));

settings.put('Defaulted__c', null);
Map<String,Object> nullFields = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(settings, true));
System.assertEquals(true, nullFields.containsKey('Defaulted__c'));
System.assertEquals(null, nullFields.get('Defaulted__c'));

Hierarchy_Setting__c named = new Hierarchy_Setting__c(Defaulted__c=false);
Map<String,Object> namedFields = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(named));
System.assertEquals(true, namedFields.containsKey('Defaulted__c'));
System.assertEquals(false, namedFields.get('Defaulted__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

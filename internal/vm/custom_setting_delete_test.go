package vm

import "testing"

func TestExecDeleteCustomSettingRejectsWrongPrefixAsInvalidID(t *testing.T) {
	program, err := CompileAnonymous(`
Hierarchy_Setting__c setting = new Hierarchy_Setting__c(Id = 'a0Bxxxxxxxxxxxx');
try {
    delete setting;
    System.assert(false, 'Expected invalid id exception.');
} catch (ListException ex) {
    System.assertEquals('Invalid id at index 0: a0Bxxxxxxxxxxxx', ex.getMessage());
}
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

func TestExecDeleteDynamicCustomSettingRejectsWrongPrefixAsInvalidID(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.SObjectType targetType = Schema.getGlobalDescribe().get('Hierarchy_Setting__c');
SObject record = targetType.newSObject();
record.put('Id', 'a0Bxxxxxxxxxxxx');
try {
    delete record;
    System.assert(false, 'Expected invalid id exception.');
} catch (ListException ex) {
    System.assertEquals('Invalid id at index 0: a0Bxxxxxxxxxxxx', ex.getMessage());
}
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

func TestExecDeleteNormalizesMissingObjectKeyPrefixBeforeValidation(t *testing.T) {
	program, err := CompileAnonymous(`
Hierarchy_Setting__c setting = new Hierarchy_Setting__c(Id = 'a0Bxxxxxxxxxxxx');
try {
    delete setting;
    System.assert(false, 'Expected invalid id exception.');
} catch (ListException ex) {
    System.assertEquals('Invalid id at index 0: a0Bxxxxxxxxxxxx', ex.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	state := org.Objects["Hierarchy_Setting__c"]
	state.Definition.KeyPrefix = ""
	org.Objects["Hierarchy_Setting__c"] = state
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

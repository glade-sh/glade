package vm

import "testing"

func TestCustomSettingMissingFieldDereference(t *testing.T) {
	program, err := CompileAnonymous(`
Local_Setting__c setting = Local_Setting__c.getValues('Missing');
System.assertEquals(null, setting);
Boolean caught = false;
try { String value = setting.Optional__c; }
catch (NullPointerException e) { caught = true; System.assertEquals('Attempt to de-reference a null object', e.getMessage()); }
System.assert(caught, 'missing setting field read must throw');
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

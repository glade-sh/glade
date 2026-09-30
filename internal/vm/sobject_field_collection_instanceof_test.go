package vm

import "testing"

func TestExecInstanceOfSObjectFieldSetRetainsTypeAcrossObject(t *testing.T) {
	methodProgram, err := CompileAnonymous(`return new Set<SObjectField>{Account.Name, Account.BillingStreet};`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Object fields = FieldHolder.getFields();
System.assert(fields instanceof Set<SObjectField>, 'sobject-field instanceof');
System.assert(!(fields instanceof Set<String>), 'string-set negative instanceof');
Set<SObjectField> typed = (Set<SObjectField>)fields;
System.assertEquals(2, typed.size());
System.assert(typed.contains(Account.Name), 'name membership');
System.assert(typed.contains(Account.BillingStreet), 'billing membership');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "FieldHolder.getFields",
		ClassName:  "FieldHolder",
		ReturnType: "Object",
		IsStatic:   true,
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

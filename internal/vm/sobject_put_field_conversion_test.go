package vm

import "testing"

func TestSObjectPutRejectsUncoercibleScalarForTypedField(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "")
	aliasSObjectPruneExecute(t, machine, `
Account invalid = new Account();
String mismatchType;
String mismatchMessage;
try { invalid.put('Name', true); }
catch (SObjectException e) { mismatchType = e.getTypeName(); mismatchMessage = e.getMessage(); }
System.assertEquals('System.SObjectException', mismatchType);
System.assertEquals('Illegal assignment from Boolean to String', mismatchMessage);
System.assertEquals(null, invalid.get('Name'));

Account numeric = new Account();
String numericType;
String numericMessage;
try { numeric.put('NumberOfEmployees', '12'); }
catch (SObjectException e) { numericType = e.getTypeName(); numericMessage = e.getMessage(); }
System.assertEquals('System.SObjectException', numericType);
System.assertEquals('Illegal assignment from String to Integer', numericMessage);
System.assertEquals(null, numeric.get('NumberOfEmployees'));

Account valid = new Account();
valid.put('Name', 'valid');
valid.put('NumberOfEmployees', 12);
System.assertEquals('valid', valid.get('Name'));
System.assertEquals(12, valid.get('NumberOfEmployees'));

Contact child = new Contact();
Account parent = new Account(Name = 'parent');
child.putSObject('Account', parent);
System.assertEquals(parent, child.getSObject('Account'));
`)
}

func TestSObjectPutRejectsUnknownFieldForSchemaObject(t *testing.T) {
	// Native API 67 scratch probe: Account.put rejects an undeclared custom field.
	machine, _ := runDynamicObjectAliasProgram(t, "")
	aliasSObjectPruneExecute(t, machine, `
Account record = new Account();
String exceptionType;
try { record.put('ProbeMissing__c', 'value'); }
catch (SObjectException e) { exceptionType = e.getTypeName(); }
System.assertEquals('System.SObjectException', exceptionType);
`)
}

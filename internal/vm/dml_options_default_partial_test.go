package vm

import "testing"

func TestDatabaseDMLOptionsDefaultReturnsSaveResultForValidationFailure(t *testing.T) {
	program, err := CompileAnonymous(`
Database.DMLOptions options = new Database.DMLOptions();
List<Database.SaveResult> results = Database.insert(new List<Account>{new Account(Bogus__c='nope')}, options);
System.assertEquals(1, results.size());
System.assertEquals(false, results[0].isSuccess());
System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE', results[0].getErrors()[0].getStatusCode());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

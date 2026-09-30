package vm

import "testing"

// Exact outcomes observed once on the authorized org with request API 67.0.
// This is a local API67 fixture, not proof of an API62-67 Salesforce interval.
func TestExecLIKEBoundAndLiteralEscapeObservation(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
String slash = '\\';
List<Account> seeds = new List<Account>{
    new Account(Name = 'A_B'), new Account(Name = 'AxB'),
    new Account(Name = 'A' + slash + '_B'), new Account(Name = 'A' + slash + 'xB')
};
insert seeds;
List<Id> ids = new List<Id>();
for (Account seed : seeds) ids.add(seed.Id);
String pattern = 'A' + slash + '_B';
String prefix = 'SELECT Name FROM Account WHERE Id IN :ids AND Name LIKE ';
List<Account> bound = Database.query(prefix + ':pattern');
System.assertEquals(1, bound.size());
System.assertEquals('A_B', bound[0].Name);
List<Account> directOne = Database.query(prefix + '\'' + pattern + '\'');
System.assertEquals(1, directOne.size());
System.assertEquals('A_B', directOne[0].Name);
String doubled = 'A' + slash + slash + '_B';
List<Account> directTwo = Database.query(prefix + '\'' + doubled + '\'');
System.assertEquals(2, directTwo.size());
Set<String> names = new Set<String>();
for (Account row : directTwo) names.add(row.Name);
System.assert(names.contains('A' + slash + '_B'));
System.assert(names.contains('A' + slash + 'xB'));
`, CompileOptions{APIVersion: "67.0"})
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

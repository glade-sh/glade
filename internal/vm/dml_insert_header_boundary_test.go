package vm

import "testing"

// These compatibility controls keep the header correction for SYSTEM_MODE list
// inserts from changing excluded overloads. Native parity for the corrected
// overload is asserted by TestSelfLookupInsertNativeConformance in apextest.
func TestDatabaseInsertHeaderExcludedOverloads(t *testing.T) {
	for name, call := range map[string]string{
		"systemSingleton": "Database.insert(bad, true, AccessLevel.SYSTEM_MODE)",
		"userList":        "Database.insert(new List<Account>{bad}, true, AccessLevel.USER_MODE)",
	} {
		t.Run(name, func(t *testing.T) {
			program, err := CompileAnonymous(`
Account bad = new Account(Bogus__c = 'unknown field');
Boolean caught = false;
try {
	` + call + `;
} catch (DmlException error) {
	caught = true;
	System.assert(error.getMessage().startsWith('Database.insert failed: '));
}
System.assertEquals(true, caught);
System.assertEquals(null, bad.Id);
System.assertEquals(0, [SELECT COUNT() FROM Account]);
`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.ExecuteInClass(program, "InsertHeaderBoundary"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

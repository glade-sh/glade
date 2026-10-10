package vm

import "testing"

// Exact API24 cases from the accepted emptyRecycleBin null-ID packet.
func TestDatabaseEmptyRecycleBinNullIDAPI24(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name:   "missingIdDefault",
			source: `Boolean thrown=false;try{Database.emptyRecycleBin(new List<Account>{new Account()});}catch(Exception e){thrown=true;System.assertEquals('System.InvalidParameterValueException',e.getTypeName());System.assertEquals('SObject passed into Database.emptyRecycleBin() had a null id.',e.getMessage());}System.assertEquals(true,thrown);`,
		},
		{
			name:   "deletedControl",
			source: `Account a=new Account(Name='purge');insert a;delete a;Database.EmptyRecycleBinResult[] r=Database.emptyRecycleBin(new List<Account>{a});System.assertEquals(1,r.size());System.assertEquals(true,r[0].isSuccess());System.assertEquals(a.Id,r[0].getId());Boolean thrown=false;try{undelete a;}catch(DmlException e){thrown=true;}System.assertEquals(true,thrown);`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(test.source, CompileOptions{APIVersion: "24.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Local-only control: a missing ID rejects the complete list before purge side effects.
func TestDatabaseEmptyRecycleBinNullIDMixedListLeavesDeletedRecord(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Account deleted = new Account(Name = 'keep');
insert deleted;
delete deleted;
Boolean caught = false;
try {
	Database.emptyRecycleBin(new List<Account>{deleted, new Account()});
} catch (InvalidParameterValueException e) {
	caught = true;
}
System.assert(caught);
System.assertEquals(1, [SELECT Id FROM Account WHERE Id = :deleted.Id ALL ROWS].size());
`, CompileOptions{APIVersion: "24.0"})
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

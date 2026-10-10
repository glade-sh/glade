package vm

import "testing"

func TestMapGetSObjectTypeCaseInsensitiveMemberCallAPI67(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Map<Id, Account> emptyAccounts = new Map<Id, Account>();
System.assertEquals(Account.SObjectType, emptyAccounts.GETSOBJECTTYPE());

Account acmeAccount = new Account(Id = '001B000001DVM9tIAH', Name = 'Acme');
Map<Id, Account> populatedAccounts = new Map<Id, Account>();
populatedAccounts.put(acmeAccount.Id, acmeAccount);
System.assertEquals(Account.SObjectType, populatedAccounts.getsobjecttype());
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

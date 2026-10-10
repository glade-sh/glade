package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSystemFieldDynamicPutPreservesTokensAndDeletionState(t *testing.T) {
	program, err := CompileAnonymous(`
Account row=new Account(Name='System fields');
System.assertEquals(null,row.IsDeleted);
Boolean rejected=false;
try {row.put(Account.IsDeleted,true);} catch(Exception e) {
 rejected=true;System.assertEquals('Field IsDeleted is not editable',e.getMessage());
}
System.assert(rejected);System.assertEquals(null,row.IsDeleted);
row.put(Account.OwnerId,UserInfo.getUserId());
insert row;
Account stored=[SELECT Id,Name,IsDeleted FROM Account WHERE Id=:row.Id];
System.assertEquals(false,stored.IsDeleted);
delete row;
Account deleted=[SELECT Id,IsDeleted FROM Account WHERE Id=:row.Id ALL ROWS];
System.assertEquals(true,deleted.IsDeleted);
try {deleted.put('isdeleted',false);System.assert(false);} catch(SObjectException e) {}
System.assertEquals(true,deleted.IsDeleted);
undelete row;
System.assertEquals(false,[SELECT IsDeleted FROM Account WHERE Id=:row.Id].IsDeleted);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	storage.EnsureStandardObjectFields(&account.Definition)
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExceptionMatchesGenericCasePreservesTypedBoundary(t *testing.T) {
	machine := New(nil)
	thrown := Object("System.SObjectException")
	for _, catchType := range []string{"Exception", "exception", "System.EXCEPTION", "SObjectException", "system.sobjectexception"} {
		if !machine.exceptionMatches(catchType, thrown) {
			t.Errorf("%s did not catch SObjectException", catchType)
		}
	}
	for _, catchType := range []string{"DmlException", "QueryException", "Custom.ExceptionLike"} {
		if machine.exceptionMatches(catchType, thrown) {
			t.Errorf("%s incorrectly caught SObjectException", catchType)
		}
	}
}

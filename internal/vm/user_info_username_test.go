package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecUserInfoUsernameCanonicalizesToLowercase(t *testing.T) {
	program, err := CompileAnonymous(`
User u = new User(
    Username = 'John.Smith@rflib.com',
    Alias = 'jsmith',
    Email = 'John.Smith@rflib.com',
    LastName = 'Smith',
    ProfileId = '00e000000000005',
    LocaleSidKey = 'en_US',
    LanguageLocaleKey = 'en_US',
    TimeZoneSidKey = 'UTC',
    EmailEncodingKey = 'UTF-8'
);
insert u;
User stored = [SELECT Id, Username FROM User WHERE Id = :u.Id];
System.runAs(stored) {
    System.assertEquals('john.smith@rflib.com', UserInfo.getUserName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

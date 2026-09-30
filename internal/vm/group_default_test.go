package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecGroupDefaultsToRegularType(t *testing.T) {
	program, err := CompileAnonymous(`
Group groupRecord = new Group(Name = 'Some Group', DeveloperName = 'Some_Group');
insert groupRecord;
Group queried = [SELECT Type FROM Group WHERE Id = :groupRecord.Id];
System.assertEquals('Regular', queried.Type);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Group")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

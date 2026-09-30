package vm

import "testing"

func TestApexPagesStandardControllerContractFamilyStorageLifecycle(t *testing.T) {
	program, err := CompileAnonymous(`
Account row = new Account(Name = 'Controller Seed');
insert row;
ApexPages.StandardController controller = new ApexPages.StandardController(row);
System.assertEquals(row.Id, controller.getId());
Account selected = (Account) controller.getRecord();
System.assertEquals(row.Id, selected.Id);
System.assertEquals('Controller Seed', selected.Name);
PageReference viewed = controller.view();
PageReference edited = controller.edit();
PageReference cancelled = controller.cancel();
System.assertNotEquals(null, viewed);
System.assertNotEquals(null, edited);
System.assertNotEquals(null, cancelled);
selected.Name = 'Controller Saved';
PageReference saved = controller.save();
System.assertNotEquals(null, saved);
System.assertEquals(row.Id, controller.getId());
Account stored = [SELECT Id, Name FROM Account WHERE Id = :row.Id];
System.assertEquals('Controller Saved', stored.Name);
PageReference deleted = controller.delete();
System.assertNotEquals(null, deleted);
List<Account> remaining = [SELECT Id FROM Account WHERE Id = :row.Id];
System.assertEquals(0, remaining.size());
`)
	if err != nil {
		t.Fatal(err)
	}

	org := testDataOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

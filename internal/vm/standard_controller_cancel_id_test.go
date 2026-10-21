package vm

import "testing"

// Owned native observations at API 62/67/68, anonymous and @IsTest, retain
// slash plus 15-character record IDs and redirect=true for cancel and view.
func TestStandardControllerNavigationObservedRecordIDs(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, testContext := range []bool{false, true} {
			context := "anonymous"
			if testContext {
				context = "test"
			}
			t.Run(api+"/"+context, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
Account row = new Account(Name='Navigation seed');
insert row;
String longID=String.valueOf(row.Id);
String shortID=longID.substring(0,15);
Account queried=[SELECT Id FROM Account WHERE Id=:row.Id];
for (Account input : new List<Account>{row,queried,new Account(Id=shortID),new Account(Id=longID)}) {
    ApexPages.StandardController controller=new ApexPages.StandardController(input);
    System.assertEquals('/'+shortID,controller.cancel().getUrl());
    System.assertEquals(true,controller.cancel().getRedirect());
    System.assertEquals('/'+shortID,controller.view().getUrl());
    System.assertEquals(true,controller.view().getRedirect());
}
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				if testContext {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Save/edit and request-only navigation are uncaptured controls. Preserve their
// local behavior until full URLs and redirect flags are observed independently.
func TestStandardControllerNavigationUncapturedControls(t *testing.T) {
	program, err := CompileAnonymous(`
Account row = new Account(Name='Navigation control');
insert row;
String longID=String.valueOf(row.Id);
ApexPages.StandardController controller=new ApexPages.StandardController(row);
System.assertEquals('/'+longID,controller.save().getUrl());
System.assertEquals('/'+longID,controller.edit().getUrl());
ApexPages.currentPage().getParameters().put('id',longID);
ApexPages.StandardController requestOnly=new ApexPages.StandardController(new Account());
System.assertEquals('/'+longID,requestOnly.cancel().getUrl());
System.assertEquals('/'+longID,requestOnly.view().getUrl());
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

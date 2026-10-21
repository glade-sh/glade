package vm

import "testing"

// Apex Developer Guide, "Conversion of Date to Datetime": Date values can
// implicitly convert to Datetime. Compare the platform argument against the
// same assignment conversion rather than introducing a timezone rule here.
func TestExecSetCreatedDateAcceptsDateArgument(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Account account = new Account(Name = 'Date argument');
insert account;
Date day = Date.today().addDays(-5);
Datetime assigned = day;
Test.setCreatedDate(account.Id, day);
Account queried = [SELECT CreatedDate FROM Account WHERE Id = :account.Id];
System.assertEquals(assigned, queried.CreatedDate);
Datetime explicitStamp = Datetime.newInstanceGmt(2026, 1, 2, 3, 4, 5);
Test.setCreatedDate(account.Id, explicitStamp);
queried = [SELECT CreatedDate FROM Account WHERE Id = :account.Id];
System.assertEquals(explicitStamp, queried.CreatedDate);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{
				Name: "CreatedDateProbe", APIVersion: api, Modifiers: []string{"@IsTest"},
				Methods: map[string]Method{"observe": {
					Name: "CreatedDateProbe.observe", ClassName: "CreatedDateProbe",
					ReturnType: "void", IsStatic: true, APIVersion: api,
					Modifiers: []string{"@IsTest"}, Program: program,
				}},
			}); err != nil {
				t.Fatal(err)
			}
			entry, err := CompileAnonymousWithOptions("CreatedDateProbe.observe();", CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(entry, "CreatedDateProbe"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

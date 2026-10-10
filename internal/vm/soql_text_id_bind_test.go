package vm

import "testing"

func TestExecSOQLTextTypedIDBindsPreserveChecksum(t *testing.T) {
	for _, tt := range []struct {
		name        string
		declaration string
		predicate   string
	}{
		{"primitive15", "Id requested='001xx000003DGbY';", "= :requested"},
		{"primitive18", "Id requested='001xx000003DGbYAAW';", "= :requested"},
		{"boxed", "Id requested=new Account(Id='001xx000003DGbY').Id;", "= :requested"},
		{"list", "List<Id> requested=new List<Id>{'001xx000003DGbY'};", "IN :requested"},
		{"set", "Set<Id> requested=new Set<Id>{'001xx000003DGbY'};", "IN :requested"},
		{"inequality", "Id requested='001xx000003DGbY';", "!= :requested"},
		{"Text case control", "String requested='001xx000003dgbyaaw';", "= :requested"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(`
Id first='001xx000003DGbY'; Id second='001xx000003DGBY';
insert new List<Account>{new Account(Name=first),new Account(Name=second)};
` + tt.declaration + `
System.assertEquals(1,[SELECT COUNT() FROM Account WHERE Name ` + tt.predicate + `]);
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
		})
	}
}

func TestExecSOQLTextTypedIDRequiresEighteenCharacters(t *testing.T) {
	// Native API 62/67 rows R013/R015/R017/R019/R029/R031/R047 in the
	// exported SOQL bind conformance cases distinguish Id binds from String.
	program, err := CompileAnonymous(`
insert new Account(Name='001000000000001');
Id requested='001000000000001';
System.assertEquals(0,[SELECT COUNT() FROM Account WHERE Name=:requested]);
Id boxed=new Account(Id=requested).Id;
System.assertEquals(0,[SELECT COUNT() FROM Account WHERE Name=:boxed]);
Set<Id> ids=new Set<Id>{requested};
System.assertEquals(0,[SELECT COUNT() FROM Account WHERE Name IN :ids]);
String text='001000000000001';
System.assertEquals(1,[SELECT COUNT() FROM Account WHERE Name=:text]);
try { Id invalid=Id.valueOf('invalid'); System.assert(false); }
catch (StringException expected) { System.assert(expected.getMessage().contains('Invalid id')); }
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

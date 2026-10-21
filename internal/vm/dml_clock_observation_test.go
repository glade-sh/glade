package vm

import (
	"testing"
	"time"
)

func TestDMLAuditTimestampsDoNotLeadCurrentTime(t *testing.T) {
	const seed = `
Account first=new Account(Name='Clock first');
insert first;
Account second=new Account(Name='Clock second');
insert second;
Datetime checkpoint=Datetime.now();
`
	for _, tc := range []struct {
		name, body string
		advance    time.Duration
	}{
		{"completed inserts precede now", `
List<Account> unexpected=[SELECT Id FROM Account WHERE LastModifiedDate>:checkpoint];
System.assertEquals(0,unexpected.size(),'completed inserts are not in the future');
`, 0},
		{"only updated subset follows checkpoint", `
first.Description='Changed';
update first;
List<Account> changed=[SELECT Id FROM Account WHERE LastModifiedDate>:checkpoint];
System.assertEquals(1,changed.size(),'unchanged rows remain before the checkpoint');
System.assertEquals(first.Id,changed[0].Id);
`, 2 * time.Second},
		{"post update now follows stored audit time", `
first.Description='Changed';
update first;
Datetime afterUpdate=Datetime.now();
List<Account> unexpected=[SELECT Id FROM Account WHERE LastModifiedDate>:afterUpdate];
System.assertEquals(0,unexpected.size(),'completed update is not in the future');
`, 0},
		{"created timestamp survives update", `
Account beforeUpdate=[SELECT CreatedDate FROM Account WHERE Id=:first.Id];
first.Description='Changed';
update first;
Account afterUpdate=[SELECT CreatedDate FROM Account WHERE Id=:first.Id];
System.assertEquals(beforeUpdate.CreatedDate,afterUpdate.CreatedDate);
`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(seed, CompileOptions{APIVersion: "61.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			// Advance a controlled clock between statements rather than making
			// an unsupported assertion about native sub-second audit resolution.
			machine.AdvanceDeterministicTime(tc.advance)
			check, err := CompileAnonymousWithOptions(tc.body, CompileOptions{APIVersion: "61.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(check); err != nil {
				t.Fatal(err)
			}
		})
	}
}

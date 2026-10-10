package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecDateTimeAndDateLiteralUseCurrentUserTimezone(t *testing.T) {
	program, err := CompileAnonymous(`
Account inside = new Account(Name = 'timezone-inside', LastSeen__c = Datetime.newInstance(2026, 5, 2, 12, 0, 0));
Account before = new Account(Name = 'timezone-before', LastSeen__c = Datetime.newInstance(2026, 5, 1, 23, 30, 0));
insert new List<Account>{inside, before};
System.assertEquals(1, [SELECT count() FROM Account WHERE Name = 'timezone-inside' AND LastSeen__c = TODAY]);
System.assertEquals(0, [SELECT count() FROM Account WHERE Name = 'timezone-before' AND LastSeen__c = TODAY]);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["LastSeen__c"] = storage.Field{APIName: "LastSeen__c", Type: storage.FieldDateTime}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{
		ID:     "005-sydney-user",
		Object: "User",
		Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue("Australia/Sydney")},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatetimeNewInstanceSupportsAsiaKolkata(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime local = Datetime.newInstance(2026, 7, 1, 12, 0, 0);
System.assertEquals('2026-07-01 06:30:00', local.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals(19800000, UserInfo.getTimeZone().getOffset(local));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetCurrentUser(storage.Record{
		ID:     "005-india-user",
		Object: "User",
		Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue("Asia/Kolkata")},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

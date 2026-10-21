package vm

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// The Apex Developer Guide's Conversion Rules support implicit Date-to-Datetime
// conversion with a zero time component. UTC avoids assuming an undocumented
// timezone interpretation at this platform-method boundary.
func TestSetCreatedDateAcceptsDateArgument(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, source := range []struct {
			name, value, expected string
		}{
			{"literal", "Date.newInstance(2026, 1, 2)", "2026-01-02 00:00:00"},
			{"todayArithmetic", "Date.today().addDays(-7)", "2026-05-08 00:00:00"},
		} {
			t.Run(api+"/"+source.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
Account account = new Account(Name = 'Backdated');
insert account;
Test.setCreatedDate(account.Id, `+source.value+`);
Account row = [SELECT Id, CreatedDate FROM Account WHERE Id = :account.Id];
System.assertEquals('`+source.expected+`', row.CreatedDate.formatGmt('yyyy-MM-dd HH:mm:ss'));
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				machine.SetCurrentUser(storage.Record{ID: "005-time-user", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue("GMT")}})
				machine.fakeNow = time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
				machine.EnableTestContext()
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSetCreatedDateRetainsDatetimeArgument(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Account account = new Account(Name = 'Backdated');
insert account;
Datetime input = Datetime.newInstanceGmt(2026, 1, 2, 3, 4, 5);
Test.setCreatedDate(account.Id, input);
Account row = [SELECT Id, CreatedDate FROM Account WHERE Id = :account.Id];
System.assertEquals(input, row.CreatedDate);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

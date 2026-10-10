package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native preservation O001-O021 and controls K021/K023: the String overload
// reads local clock fields, the Object overload retains a Datetime or null.
func TestDatetimeValueOfUsesLocalClockAndRejectsISO(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Datetime.newInstance(2026,5,2,5,0,0),Datetime.valueOf('2026-05-02 05:00:00Z'));
System.assertEquals(Datetime.newInstance(2026,5,2,7,0,0),Datetime.valueOf('2026-05-02 07:00:00+02:00'));
System.assertEquals(Datetime.newInstance(2026,5,2,5,0,0),Datetime.valueOf('2026-05-02 05:00:00'));
System.assertEquals(Datetime.newInstance(2026,5,2,5,0,0),Datetime.valueOf('2026-05-02 5:0:0'));
System.assertEquals(Datetime.newInstanceGmt(2026,5,2,7,0,0),Datetime.valueOfGmt('2026-05-02 07:00:00+02:00'));
String rejected='';
try { Datetime.valueOf('2026-05-02T05:00:00Z'); } catch(TypeException e) { rejected=e.getMessage(); }
System.assertEquals('Invalid date/time: 2026-05-02T05:00:00Z',rejected);
rejected='';
try { Datetime.valueOfGmt('2026-05-02T05:00:00Z'); } catch(TypeException e) { rejected=e.getMessage(); }
System.assertEquals('Invalid date/time: 2026-05-02T05:00:00Z',rejected);
Object n;
System.assertEquals(null,Date.valueOf(n));
System.assertEquals(null,Datetime.valueOf(n));
Datetime d=Datetime.newInstanceGmt(2026,5,2,1,2,3);
Object o=d;
System.assertEquals(d,Datetime.valueOf(o));
o='2026-05-02 01:02:03';
rejected='';
try { Datetime.valueOf(o); } catch(TypeException e) { rejected=e.getMessage(); }
System.assertEquals('Invalid date/time: 2026-05-02 01:02:03',rejected);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetCurrentUser(storage.Record{ID: "005-zone-user", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue("America/Los_Angeles")}})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

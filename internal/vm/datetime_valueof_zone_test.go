package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestDatetimeValueOfLocalZonePreservesExplicitOffsets(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime utc=Datetime.valueOfGmt('2026-05-02 05:00:00');
System.assertEquals(utc,Datetime.valueOf('2026-05-02 05:00:00Z'));
System.assertEquals(utc,Datetime.valueOf('2026-05-02T05:00:00Z'));
System.assertEquals(utc,Datetime.valueOf('2026-05-02 07:00:00+02:00'));
System.assertEquals(Datetime.newInstance(2026,5,2,5,0,0),Datetime.valueOf('2026-05-02 05:00:00'));
System.assertEquals(Datetime.newInstance(2026,5,2,5,0,0),Datetime.valueOf('2026-05-02 5:0:0'));
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

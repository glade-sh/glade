package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestDynamicDatetimeAssignmentRejectsString(t *testing.T) {
	program, err := CompileAnonymous(`
Event row=new Event();
// String rejection is a pending native probe. Retain documented typed/null controls.
Datetime instant=Datetime.newInstance(1767225600000L);
row.put(Event.StartDateTime,instant);
System.assertEquals(instant,row.StartDateTime);
row.put(Event.StartDateTime,null);
System.assertEquals(null,row.StartDateTime);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Event")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// String rejection is a pending native boundary; null and Datetime stay valid.
func TestDynamicDatetimePutRejectsString(t *testing.T) {
	p, e := CompileAnonymous(`Event row=new Event();Boolean rejected=false;try{row.put(Event.StartDateTime,'no instant');}catch(SObjectException ex){rejected=true;}System.assert(rejected);row.put('startdatetime',null);Datetime value=Datetime.newInstance(1767225600000L);row.put(Event.StartDateTime,value);System.assertEquals(value,row.get('StartDateTime'));`)
	if e != nil {
		t.Fatal(e)
	}
	m := New(nil)
	o := testDataOrg()
	storage.EnsureStandardObject(&o, "Event")
	m.SetOrg(&o)
	if _, e = m.Execute(p); e != nil {
		t.Fatal(e)
	}
}

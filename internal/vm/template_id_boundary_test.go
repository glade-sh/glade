package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Unpersisted CaseComment resolution is pending native template/normal controls.
func TestGeneratedTemplateIDTypeBoundary(t *testing.T) {
	p, e := CompileAnonymous(`Id recordId=Id.valueOf(CaseComment.SObjectType.getDescribe().getKeyPrefix()+'000000000001');Boolean rejected=false;try{recordId.getSObjectType();}catch(SObjectException ex){rejected=true;}System.assert(rejected);System.assertEquals(Contact.SObjectType,Id.valueOf('003000000000001').getSObjectType());`)
	if e != nil {
		t.Fatal(e)
	}
	m := New(nil)
	o := testDataOrg()
	storage.EnsureStandardObject(&o, "CaseComment")
	storage.EnsureStandardObject(&o, "Contact")
	m.SetOrg(&o)
	if _, e = m.Execute(p); e != nil {
		t.Fatal(e)
	}
}

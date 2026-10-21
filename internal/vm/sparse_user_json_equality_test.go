package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSparseUserJSONRoundTripMatchesSuppliedFields(t *testing.T) {
	p, e := CompileAnonymous(`User before=new User(Id=UserInfo.getUserId(),Username=UserInfo.getUsername());SObject after=(SObject)JSON.deserialize(JSON.serialize(before),SObject.class);System.assertEquals(before,after);`)
	if e != nil {
		t.Fatal(e)
	}
	m := New(nil)
	o := testDataOrg()
	storage.EnsureStandardObject(&o, "User")
	m.SetOrg(&o)
	if _, e = m.Execute(p); e != nil {
		for _, n := range []string{"before", "after"} {
			v := m.Globals[n]
			t.Logf("%s type=%s", n, v.Type)
			for k, x := range v.Fields {
				if sObjectEqualityFieldIncluded(v, k, x) {
					t.Logf("%s %s=%#v", n, k, x)
				}
			}
		}
		t.Fatal(e)
	}
}

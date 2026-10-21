package vm

import "testing"

// id_uuid_version/cases.json captures scalar Id String.valueOf normalization.
// Assert local scalar/collection consistency; nested native cells are pending.
func TestExecNestedCollectionIdDisplayConsistency(t *testing.T) {
	program, err := CompileAnonymous(`
Id value='001000000000001';
Set<Id> ids=new Set<Id>{value};
Map<String,Object> boxed=new Map<String,Object>{'items'=>ids};
System.assertEquals('{items='+String.valueOf(ids)+'}',String.valueOf(boxed));
List<Id> listIds=new List<Id>{value};
boxed.put('items',listIds);
System.assertEquals('{items='+String.valueOf(listIds)+'}',String.valueOf(boxed));
Set<String> text=new Set<String>{'001000000000001'};
boxed.put('items',text);
System.assertEquals('{items={001000000000001}}',String.valueOf(boxed));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

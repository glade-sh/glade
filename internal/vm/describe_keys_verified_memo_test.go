package vm

import (
	"reflect"
	"testing"
)

// Repeated getGlobalDescribe calls on a shared backing map reuse the verified
// key metadata. The reused keys must equal a fresh derivation, a written key
// must not reach the next call, and a metadata reset must drop the memo.
func TestGlobalDescribeKeysVerifiedMemo(t *testing.T) {
	base, template := describeSharePrimedBaseFor(t, false)
	machine := describeShareClone(base, template)
	first := machine.globalDescribeMapValue()
	if !first.mapBackingShared {
		t.Skip("route does not extend the global map; memo not applicable")
	}
	second := machine.globalDescribeMapValue()
	if !sameMapBacking(first.Map, second.Map) || !sameMapBacking(first.MapKeys, second.MapKeys) {
		t.Fatal("repeated call rebuilt shared key metadata")
	}
	fresh := Value{Map: second.Map}
	fillDescribeMapStoredKeys(&fresh)
	if !reflect.DeepEqual(second.MapKeys, fresh.MapKeys) || !reflect.DeepEqual(second.MapOrder, fresh.MapOrder) {
		t.Fatal("memoized keys differ from a fresh derivation")
	}
	size := len(second.Map)
	machine.Globals["held"] = second
	if _, _, err := machine.callValueMember("held", second, "put", []Value{String("private"), sObjectTypeToken("Account")}, &Result{}); err != nil {
		t.Fatal(err)
	}
	// The write persists in the VM's raw map (compatibility baseline); the
	// next call must see it in its keys, never the memoized older key set.
	next := machine.globalDescribeMapValue()
	if len(next.Map) != len(next.MapKeys) || len(next.MapOrder) != len(next.Map) {
		t.Fatalf("stale keys after write: map=%d keys=%d order=%d", len(next.Map), len(next.MapKeys), len(next.MapOrder))
	}
	if _, ok := next.MapKeys[mapKey(String("private"))]; !ok {
		t.Fatal("memoized keys hid a written entry")
	}
	if len(second.MapKeys) != size {
		t.Fatal("new key set changed a retained holder")
	}
	machine.clearMetadataCaches()
	if machine.globalDescribeKeysVerified.keys != nil {
		t.Fatal("metadata reset kept the verified key memo")
	}
}

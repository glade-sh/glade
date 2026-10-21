package vm

import "testing"

// Owned hierarchy_exception_native C02-01 observes fresh getOrgDefaults
// references. Cache internals are not the public accessor contract.
func TestHierarchyAccessorCacheReturnsDetachedValues(t *testing.T) {
	m := New(nil)
	o := customDataOrg()
	m.SetOrg(&o)
	read := func() Value {
		t.Helper()
		value, handled, err := m.callCustomDataStaticMember("Hierarchy_Setting__c", "getOrgDefaults", nil)
		if err != nil || !handled || value.Kind != ValueObject {
			t.Fatalf("read: value=%v handled=%v err=%v", value, handled, err)
		}
		return value
	}
	first := read()
	first.Fields["Id"] = Null
	next := read()
	if next.Fields["Id"].Kind == ValueNull {
		t.Fatal("caller changed org default snapshot")
	}
	next.Fields["Enabled__c"] = Bool(false)
	again := read()
	if !again.Fields["Enabled__c"].Bool {
		t.Fatal("org default accessor results alias")
	}
}

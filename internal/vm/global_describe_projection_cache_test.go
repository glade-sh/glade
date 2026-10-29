package vm

import (
	"testing"
)

// Share immutable projection keys within a frozen generation, while rebuilding
// values from the current raw map and synthesizing call-local token payloads.
func TestGlobalDescribeProjectionSharesOnlyImmutableKeys(t *testing.T) {
	base, template := describeSharePrimedBaseFor(t, true)
	machine := describeShareClone(base, template)

	first := machine.globalDescribeMapValue()
	second := machine.globalDescribeMapValue()
	if !isGlobalDescribeMap(first) || len(first.Map) == 0 {
		t.Fatalf("projection shape: runtime=%q size=%d", first.Runtime, len(first.Map))
	}
	if sameMapBacking(first.Map, second.Map) {
		t.Fatal("separate calls share mutable projection values")
	}
	if !sameMapBacking(first.MapKeys, second.MapKeys) {
		t.Fatal("separate calls rebuilt immutable projection keys")
	}
	if sameMapBacking(first.Map, machine.schemaGlobalDescribe().Map) {
		t.Fatal("projection shares the raw describe storage")
	}
	view, err := machine.coerceAssignable("Map<String,Schema.SObjectType>", second)
	if err != nil {
		t.Fatal(err)
	}
	if view.Ref == second.Ref {
		t.Fatal("typed describe exposed the cached projection storage")
	}
	if len(view.Map) != len(second.Map) {
		t.Fatalf("typed view size %d, want %d", len(view.Map), len(second.Map))
	}
}

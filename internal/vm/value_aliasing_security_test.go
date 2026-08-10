package vm

import "testing"

func TestPropagateTopLevelAliasSnapshotToScopeCountUsesUnsignedCount(t *testing.T) {
	original := Value{Kind: ValueList, Type: "List<String>", Ref: 7}
	updated := Value{Kind: ValueList, Type: "List<String>", Ref: 8}
	scope := map[string]Value{
		"first":  original,
		"second": original,
	}

	var got uint64 = propagateTopLevelAliasSnapshotToScopeCount(scope, snapshotAlias(original), updated)
	if got != 2 {
		t.Fatalf("changed count = %d, want 2", got)
	}
	for name, value := range scope {
		if value.Ref != updated.Ref {
			t.Fatalf("scope[%q].Ref = %d, want %d", name, value.Ref, updated.Ref)
		}
	}
}

func TestPropagateMethodReturnMapMutationPreservesMapOrder(t *testing.T) {
	machine := New(nil)
	original := Map()
	original.Type = "Map<Integer,String>"
	machine.Globals["values"] = original
	previous := snapshotAlias(original)

	updated := original
	key := mapKey(Int(0))
	updated.Map[key] = String("value")
	updated.MapKeys[key] = Int(0)
	updated.MapOrder = append(updated.MapOrder, key)
	if sameAliasRuntimeData(original, updated) {
		t.Fatal("map order mutation must not compare equal")
	}

	if !machine.propagateMethodReturnAliasSnapshotMutationToScope(machine.Globals, previous, original, updated, true) {
		t.Fatal("expected method return mutation to update the caller map")
	}
	if got := machine.Globals["values"].MapOrder; len(got) != 1 || got[0] != key {
		t.Fatalf("caller map order = %#v, want %#v", got, []string{key})
	}
}

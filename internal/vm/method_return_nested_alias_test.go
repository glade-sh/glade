package vm

import "testing"

func TestMethodReturnRefreshesDirectAndNestedListAlias(t *testing.T) {
	machine := New(nil)
	original := testTypedList("List<String>")
	unrelated := testTypedList("List<String>")
	container := Map()
	container.Type = "Map<String,List<String>>"
	key := mapKey(String("ladder"))
	container.Map[key] = original
	container.MapKeys[key] = String("ladder")
	scope := map[string]Value{"direct": original, "nested": container, "unrelated": unrelated}
	updated := original
	updated.List = []Value{String("first")}
	machine.propagateMethodReturnAliasSnapshotMutationToScope(scope, snapshotAlias(original), original, updated, true)
	if len(scope["direct"].List) != 1 || len(scope["nested"].Map[key].List) != 1 {
		t.Fatal("method return refreshed only one of the direct/nested aliases")
	}
	if scope["direct"].Ref != original.Ref || scope["nested"].Map[key].Ref != original.Ref || len(scope["unrelated"].List) != 0 {
		t.Fatal("reference identity or unrelated collection changed")
	}
}

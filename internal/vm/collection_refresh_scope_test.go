package vm

import "testing"

func TestCollectionRefreshSkipsUnsharedDescendants(t *testing.T) {
	machine := New(nil)
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	rows := List()
	rows.Type = "List<List<Integer>>"
	for i := range 32 {
		rows.List = append(rows.List, List(Int(int64(i))))
	}
	owner := Object("Basket")
	owner.Fields["rows"] = rows
	other := Object("Holder")
	other.Fields["values"] = List(Int(9))
	scope := map[string]Value{"basket": owner, "other": other}
	machine.propagateUpdatedValueAliases(scope, owner)
	stats := recorder.Snapshot().ScopeAlias
	if stats.Calls > 1 {
		t.Fatalf("unshared descendants caused %d scope refreshes; want at most 1", stats.Calls)
	}
}

func TestCollectionRefreshRetainsEscapedDescendant(t *testing.T) {
	machine := New(nil)
	before := List(Int(0))
	after := before
	after.List = append(append([]Value(nil), before.List...), Int(100))
	owner := Object("Basket")
	owner.Fields["rows"] = List(after)
	holder := Object("Holder")
	holder.Fields["values"] = before
	scope := map[string]Value{"basket": owner, "holder": holder}
	machine.propagateUpdatedValueAliases(scope, owner)
	if got := scope["holder"].Fields["values"].List; len(got) != 2 || got[1].Int != 100 {
		t.Fatalf("escaped descendant = %#v; want [0,100]", got)
	}
}

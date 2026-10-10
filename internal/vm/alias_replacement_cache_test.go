package vm

import (
	"strconv"
	"testing"
)

func aliasReplacementWideScope() (*VM, map[string]Value, Value, Value) {
	machine := New(nil)
	target := testTypedList("List<Object>", String("before"))
	updated := target
	updated.List = append(append([]Value(nil), target.List...), String("after"))
	roots := testTypedList("List<Object>")
	for range 8 {
		records := testTypedList("List<SObject>")
		for i := 0; i < 128; i++ {
			record := Object("OwnedRecord__c")
			for f := 0; f < 12; f++ {
				record.Fields["Field"+strconv.Itoa(f)] = String("value")
			}
			records.List = append(records.List, record)
		}
		wrapper := Object("Bucket")
		wrapper.Fields["Rows"] = records
		roots.List = append(roots.List, wrapper)
	}
	roots.List = append(roots.List, target)
	return machine, map[string]Value{"root": roots}, target, updated
}

func BenchmarkAliasReplacementWideScope(b *testing.B) {
	machine, scope, target, updated := aliasReplacementWideScope()
	previous := snapshotAlias(target)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine.advanceAliasContainmentMutation()
		machine.propagateAliasSnapshotToScope(scope, previous, updated)
	}
}

func TestAliasReplacementWideScopePreservesRowsAndTarget(t *testing.T) {
	machine, scope, target, updated := aliasReplacementWideScope()
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), updated)
	roots := scope["root"].List
	if len(roots[8].List) != 2 || roots[8].Ref != target.Ref {
		t.Fatal("target alias not refreshed")
	}
	for _, root := range roots[:8] {
		rows := root.Fields["Rows"].List
		if len(rows) != 128 {
			t.Fatal("unrelated row count changed")
		}
		for _, row := range rows {
			if len(row.Fields) != 12 || row.Fields["Field0"].Text != "value" {
				t.Fatal("unrelated row changed")
			}
		}
	}
}

func TestAliasReplacementRechecksInvalidatedNegativeBranch(t *testing.T) {
	machine, scope, target, updated := aliasReplacementWideScope()
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), updated)
	// A previously absent branch now contains the target through an object field.
	// The same mutation epoch used by containment must also guard replacement.
	rows := scope["root"].List[0].Fields["Rows"]
	rows.List[0].Fields["Nested"] = target
	machine.advanceAliasContainmentMutation()
	updated.List = append(updated.List, String("third"))
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), updated)
	nested := scope["root"].List[0].Fields["Rows"].List[0].Fields["Nested"]
	if nested.Ref != target.Ref || len(nested.List) != 3 {
		t.Fatal("stale miss hid a newly attached alias")
	}
	if len(scope["root"].List[8].List) != 3 {
		t.Fatal("top-level sibling alias not updated")
	}
}

package vm

import (
	"fmt"
	"testing"
)

func TestAliasContainmentSkipsScalarListChildren(t *testing.T) {
	for _, width := range []int{200, 800} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			machine := New(nil)
			target := List(String("before"))
			target.Type = "List<Object>"
			previous := snapshotAlias(target)
			updated := target
			updated.List = append(append([]Value(nil), target.List...), String("after"))

			root := List()
			root.Type = "List<Object>"
			root.List = make([]Value, 0, width+1)
			for range width {
				root.List = append(root.List, String("noise"))
			}
			root.List = append(root.List, target)
			machine.Globals["root"] = root

			recorder := NewPerfRecorder()
			machine.SetPerfRecorder(recorder)
			machine.propagateAliasSnapshotToScope(machine.Globals, previous, updated)

			if visits := recorder.Snapshot().ScopeAlias.RecursiveVisits; visits > 2 {
				t.Fatalf("containment visits = %d for %d scalar children; want <= 2", visits, width)
			}
			got := machine.Globals["root"].List[width]
			if got.Ref != target.Ref || len(got.List) != 2 || got.List[1].Text != "after" {
				t.Fatalf("nested alias = %#v; want refreshed backing for ref %d", got, target.Ref)
			}
		})
	}
}

func TestAliasContainmentRetainsScalarAliasRef(t *testing.T) {
	machine := New(nil)
	target := Value{Kind: ValueString, Text: "before", Ref: 42}
	root := List(target)
	previous := snapshotAlias(target)

	if !machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
		t.Fatal("containment missed a scalar value with the target alias ref")
	}
	updated := target
	updated.Text = "after"
	replaced, changed := replaceValueAliasRef(root, previous, updated, make(map[uint64]bool))
	if !changed || replaced.List[0].Text != "after" {
		t.Fatalf("replacement = %#v, changed = %t; want updated scalar alias", replaced.List[0], changed)
	}
}

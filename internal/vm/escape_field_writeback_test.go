package vm

import (
	"runtime"
	"testing"
)

func TestScalarListWritebackWithoutTraversalAllocation(t *testing.T) {
	machine := New(nil)
	machine.rememberLocalOnlyObject(Object("Pending"))
	previous := List(Int(1), Int(2), Int(3))
	updated := previous
	updated.List = append(updated.List, Int(4))
	for range 3 {
		// Empty the traversal pool before counting only the writeback itself.
		runtime.GC()
		runtime.GC()
		warmup := true
		allocs := testing.AllocsPerRun(1, func() {
			if warmup {
				warmup = false
				return
			}
			machine.markCollectionRefsEscapedSince(previous, updated)
		})
		if allocs != 0 {
			t.Fatalf("scalar list writeback allocated %v objects; want none", allocs)
		}
	}
}

func TestListWritebackVisitsUnreferencedContainer(t *testing.T) {
	machine := New(nil)
	child := Object("Child")
	machine.rememberLocalOnlyObject(child)
	previous := List(Int(1))
	updated := previous
	updated.List = append(updated.List, Value{Kind: ValueList, List: []Value{child}})
	machine.markCollectionRefsEscapedSince(previous, updated)
	if machine.localOnlyAlias(snapshotAlias(child)) {
		t.Fatal("nested child remained local-only after writeback")
	}
}

func TestListWritebackRetainsSeenScalarReferences(t *testing.T) {
	machine := New(nil)
	child := Object("Child")
	machine.rememberLocalOnlyObject(child)
	previous := List()
	updated := previous
	updated.List = append(updated.List, Value{Kind: ValueInt, Ref: child.Ref}, child)
	machine.markCollectionRefsEscapedSince(previous, updated)
	if !machine.localOnlyAlias(snapshotAlias(child)) {
		t.Fatal("a scalar reference must participate in the visited set")
	}
}

func TestListWritebackMarksReplacedPrefixSlot(t *testing.T) {
	machine := New(nil)
	child := Object("Child")
	machine.rememberLocalOnlyObject(child)
	previous := List(Int(1), Int(2))
	updated := previous
	updated.List = []Value{Int(1), child}
	machine.markCollectionRefsEscapedSince(previous, updated)
	if machine.localOnlyAlias(snapshotAlias(child)) {
		t.Fatal("object stored over a scalar slot remained local-only")
	}
}

func TestSetAndMapWritebackMarkNewNestedObjects(t *testing.T) {
	machine := New(nil)
	a, b := Object("A"), Object("B")
	machine.rememberLocalOnlyObject(a)
	machine.rememberLocalOnlyObject(b)
	set := Value{Kind: ValueSet, Ref: 9001}
	set.Set = []Value{a}
	m := Value{Kind: ValueMap, Ref: 9002, Map: map[string]Value{"k": b}}
	machine.markCollectionRefsEscapedSince(Value{Kind: ValueSet, Ref: 9001}, set)
	machine.markCollectionRefsEscapedSince(Value{Kind: ValueMap, Ref: 9002, Map: map[string]Value{}}, m)
	if machine.localOnlyAlias(snapshotAlias(a)) || machine.localOnlyAlias(snapshotAlias(b)) {
		t.Fatal("nested set or map element remained local-only")
	}
}

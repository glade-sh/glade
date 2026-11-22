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
	allocs := testing.AllocsPerRun(3, func() {
		// Empty the traversal pool to measure the cold path as well.
		runtime.GC()
		runtime.GC()
		machine.markCollectionRefsEscapedSince(previous, updated)
	})
	if allocs != 0 {
		t.Fatalf("scalar list writeback allocated %v objects; want none", allocs)
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

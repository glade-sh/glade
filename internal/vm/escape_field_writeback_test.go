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

func TestListWritebackSharedBackingScansOnlyTail(t *testing.T) {
	machine := New(nil)
	child := Object("Child")
	machine.rememberLocalOnlyObject(child)
	previous := List(Int(1), Int(2))
	previous.List = append(make([]Value, 0, 8), previous.List...)
	updated := previous
	updated.List = append(updated.List, Value{Kind: ValueList, List: []Value{child}})
	machine.markCollectionRefsEscapedSince(previous, updated)
	if machine.localOnlyAlias(snapshotAlias(child)) {
		t.Fatal("nested child in appended tail remained local-only")
	}
}

// A two-segment local field target should cost the same to write as its
// already-split path. Raster list appends write this target for each pixel.
func TestLocalFieldWritebackDoesNotAllocatePath(t *testing.T) {
	machine := New(nil)
	holder := Object("PixelBuffer")
	holder.Fields["pixel"] = Int(0)
	machine.Globals["buffer"] = holder

	direct := testing.AllocsPerRun(100, func() {
		if err := machine.assignPath(holder, []string{"pixel"}, Int(6)); err != nil {
			t.Fatal(err)
		}
	})
	dotted := testing.AllocsPerRun(100, func() {
		if err := machine.assignValue("buffer.pixel", Int(7), false); err != nil {
			t.Fatal(err)
		}
	})
	if pixel := machine.Globals["buffer"].Fields["pixel"]; pixel.Kind != ValueInt || pixel.Int != 7 {
		t.Fatalf("stored pixel = %v, want 7", pixel)
	}
	if dotted > direct {
		t.Fatalf("dotted writeback allocated %.0f times, split path allocated %.0f times", dotted, direct)
	}
}

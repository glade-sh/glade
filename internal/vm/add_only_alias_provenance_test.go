package vm

import (
	"strconv"
	"testing"
)

func TestAddOnlyAliasMutationPreservesUnindexedSharedParentsAndCycle(t *testing.T) {
	machine := New(nil)
	container := testTypedList("List<Object>", String("before"))

	shared := Object("SharedParent")
	shared.Fields["Items"] = container
	shared.Fields["Self"] = shared
	caller := Object("CallerParent")
	caller.Fields["Shared"] = shared
	other := Map()
	other.Type = "Map<String,Object>"
	otherKey := mapKey(String("shared"))
	other.Map[otherKey] = shared
	other.MapKeys[otherKey] = String("shared")
	other.MapOrder = []string{otherKey}
	machine.Globals["caller"] = caller
	machine.Globals["other"] = other
	machine.Classes["StaticOwner"] = Class{
		Name: "StaticOwner",
		StaticFields: map[string]Field{
			"Shared": {Name: "Shared", Type: "SharedParent", Static: true, Value: shared},
		},
	}

	added := Object("AddedValue")
	updated := container
	updated.List = append(append([]Value(nil), container.List...), added)
	machine.propagateCollectionMutationFromSnapshot(snapshotAlias(container), updated)

	callerShared := machine.Globals["caller"].Fields["Shared"]
	otherShared := machine.Globals["other"].Map[otherKey]
	staticShared := machine.Classes["StaticOwner"].StaticFields["Shared"].Value
	for name, got := range map[string]Value{
		"caller": callerShared,
		"other":  otherShared,
		"static": staticShared,
	} {
		items := got.Fields["Items"]
		if items.Ref != container.Ref || len(items.List) != 2 || items.List[1].Ref != added.Ref {
			t.Fatalf("%s add-only alias = %#v, want added value through original container ref", name, items)
		}
		self := got.Fields["Self"]
		if self.Ref != shared.Ref || !sameMapBacking(self.Fields, got.Fields) {
			t.Fatalf("%s cycle changed during add-only fallback: shared=%#v self=%#v", name, got, self)
		}
	}
}

func TestAddOnlyAliasMutationStillVisitsEveryUnindexedCallerRoot(t *testing.T) {
	ResetPerfCounters()
	SetPerfCountersEnabled(true)
	t.Cleanup(ResetPerfCounters)

	machine := New(nil)
	const rootCount = 128
	container := testTypedList("List<Object>", String("before"))
	for i := 0; i < rootCount; i++ {
		root := Object("Parent")
		root.Fields["Items"] = container
		machine.Globals["root-"+strconv.Itoa(i)] = root
	}

	updated := container
	updated.List = append(append([]Value(nil), container.List...), Object("AddedValue"))
	machine.propagateCollectionMutationFromSnapshot(snapshotAlias(container), updated)

	if got := SnapshotPerfCounters().ScopeAlias.Roots; got != rootCount {
		t.Fatalf("add-only mutation visited %d caller roots, want %d without a complete parent-location index", got, rootCount)
	}
}

func BenchmarkAddOnlyAliasPropagationWithoutParentIndex(b *testing.B) {
	for _, rootCount := range []int{1, 128, 1024} {
		b.Run(strconv.Itoa(rootCount)+"-roots", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				machine := New(nil)
				container := testTypedList("List<Object>", String("before"))
				for rootIndex := 0; rootIndex < rootCount; rootIndex++ {
					root := Object("Parent")
					root.Fields["Items"] = container
					machine.Globals["root-"+strconv.Itoa(rootIndex)] = root
				}
				updated := container
				updated.List = append(append([]Value(nil), container.List...), Object("AddedValue"))
				previous := snapshotAlias(container)
				b.StartTimer()

				machine.propagateCollectionMutationFromSnapshot(previous, updated)
			}
		})
	}
}

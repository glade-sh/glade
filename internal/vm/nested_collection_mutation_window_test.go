package vm

import "testing"

func TestNestedCollectionMutationWindowTracksNestedBackings(t *testing.T) {
	for _, kind := range []string{"list", "set", "map", "map-list"} {
		t.Run(kind, func(t *testing.T) {
			before := List(String("before"))
			after := before
			after.List = []Value{String("before"), String("after")}
			if kind == "set" {
				before = Set(String("before"))
				after = before
				after.Set = []Value{String("before"), String("after")}
			} else if kind == "map" {
				before = Map()
				after = before
				after.Map = map[string]Value{"added": String("after")}
			}
			receiver := Object("MutationReader")
			receiver.Fields["value"] = after
			if kind == "map-list" {
				container := Map()
				container.Map["nested"] = after
				receiver.Fields["value"] = container
			}
			caller := Object("MutationHolder")
			caller.Fields["value"] = before
			caller.Fields["cycle"] = caller
			scope := map[string]Value{"holder": caller}
			machine := New(nil)
			since := machine.collectionMutationSeq
			machine.propagateCollectionMutation(before, after)
			if machine.nestedCollectionsUnchangedSince(receiver, since, nil) {
				t.Fatal("mutated nested backing was missed")
			}
			machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(receiver), receiver, receiver, true, since)
			if !sameMethodReturnAliasBatchBacking(scope["holder"].Fields["value"], after) {
				t.Fatal("mutated collection did not reach the cyclic caller holder")
			}
		})
	}
}

func TestNestedCollectionMutationWindowIsExplicit(t *testing.T) {
	machine := New(nil)
	values := List(String("synthetic"))
	receiver := Object("WindowReader")
	receiver.Fields["value"] = values
	machine.collectionMutationSeq = 4
	machine.recordCollectionMutation(values.Ref)
	for _, tc := range []struct {
		since     uint64
		unchanged bool
	}{{3, false}, {4, true}, {3, false}} {
		if got := machine.nestedCollectionsUnchangedSince(receiver, tc.since, nil); got != tc.unchanged {
			t.Fatalf("since=%d unchanged=%v; want %v", tc.since, got, tc.unchanged)
		}
	}
}

func TestNestedCollectionMutationWindowKeepsDivergentReceiverSnapshots(t *testing.T) {
	machine := New(nil)
	values := List(String("synthetic"))
	current := Object("SnapshotBranch")
	current.Fields["value"] = values
	stale := current
	stale.Fields = map[string]Value{"value": Null}
	// The ordered first occurrence cannot hide another snapshot's descendants.
	receiver := List(stale, current)
	since := machine.collectionMutationSeq
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(values.Ref)
	if machine.nestedCollectionsUnchangedSince(receiver, since, nil) {
		t.Fatal("divergent receiver snapshot hid a mutated reachable collection")
	}
}

func TestNestedCollectionMutationWindowBoundsSharedSetTraversal(t *testing.T) {
	const width = 200
	items := make([]Value, width)
	for i := range items {
		items[i] = Int(int64(i))
	}
	shared := Set(items...)
	shared.setInsertionHashes = make([]setInsertionHash, width)
	aliases := make([]Value, width)
	for i := range aliases {
		aliases[i] = shared
	}
	machine := New(nil)
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(newValueRef())
	var visits uint64
	if !machine.nestedCollectionsUnchangedSince(List(aliases...), 0, &visits) {
		t.Fatal("unchanged shared Set required a caller scan")
	}
	if visits > 3*width+8 {
		t.Fatalf("shared Set graph rescanned per occurrence: visits=%d", visits)
	}
}

func TestNestedCollectionMutationWindowPruningFallsBack(t *testing.T) {
	machine := New(nil)
	values := List(String("synthetic"))
	receiver := Object("PrunedReader")
	receiver.Fields["value"] = values
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(values.Ref)
	for i := 0; i < collectionMutationRefSeqMaxEntries; i++ {
		machine.collectionMutationSeq++
		machine.recordCollectionMutation(newValueRef())
	}
	if machine.collectionMutationRefFloor == 0 || len(machine.collectionMutationRefSeq) > collectionMutationRefSeqMaxEntries {
		t.Fatal("mutation history was not bounded and pruned")
	}
	if machine.nestedCollectionsUnchangedSince(receiver, 0, nil) {
		t.Fatal("pruned mutation window skipped the conservative scan")
	}
	if !machine.nestedCollectionsUnchangedSince(receiver, machine.collectionMutationRefFloor, nil) {
		t.Fatal("mutation completed before the floor-equal window was treated as new")
	}
}

func TestNestedCollectionMutationWindowCloneIsolation(t *testing.T) {
	machine := New(nil)
	values := List(String("synthetic"))
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(values.Ref)
	clone := machine.CloneRuntime(nil)
	if clone.collectionMutationSeq != 0 || clone.collectionMutationRefFloor != 0 || len(clone.collectionMutationRefSeq) != 0 {
		t.Fatal("clone inherited another VM's mutation window")
	}
	clone.collectionMutationSeq = 2
	clone.recordCollectionMutation(values.Ref)
	if machine.collectionMutationRefSeq[values.Ref] != 1 {
		t.Fatal("clone shared mutation history storage")
	}
}

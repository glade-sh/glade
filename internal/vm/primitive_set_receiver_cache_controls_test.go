package vm

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestPrimitiveSetMembershipCloneRemoveAndTransportRoutes(t *testing.T) {
	machine := New(nil)
	set, err := machine.constructTrackedSet("Set<String>", []Value{String("Alpha"), String("alpha"), Null}, true, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.setContainsValue(set, String("Alpha"), &Result{})
	if err != nil {
		t.Fatal(err)
	}
	for _, clone := range []Value{cloneValuePreserveRefs(set), cloneValueDetachedPreserveRefs(set)} {
		clone = appendSetEntry(clone, String("clone"), setInsertionHash{})
		found, _ := machine.setContainsValue(set, String("clone"), &Result{})
		if found {
			t.Fatal("clone updated original index")
		}
		found, _ = machine.setContainsValue(clone, String("Alpha"), &Result{})
		if !found {
			t.Fatal("clone lost original membership")
		}
	}
	removed := removeSetEntry(set, 0)
	for _, tc := range []struct {
		value Value
		key   Value
		want  bool
	}{{set, String("Alpha"), true}, {removed, String("Alpha"), false}, {removed, String("alpha"), true}, {removed, Null, true}} {
		got, err := machine.setContainsValue(tc.value, tc.key, &Result{})
		if err != nil || got != tc.want {
			t.Fatalf("membership=%v want=%v error=%v", got, tc.want, err)
		}
	}
	transported, err := machine.coerceAssignable("Set<Object>", set)
	if err != nil {
		t.Fatal(err)
	}
	found, err := machine.setContainsValue(transported, String("alpha"), &Result{})
	if err != nil || !found {
		t.Fatal("Object view lost membership")
	}
}

func TestPrimitiveSetAppendRejectedStoreSnapshotRoute(t *testing.T) {
	original := Set()
	original.Type = "Set<String>"
	for _, text := range []string{"first", "second", "third"} {
		original = appendSetEntry(original, String(text), setInsertionHash{})
	}
	// A failed caller can discard the new Value. Its old prefix and history
	// remain usable, including a later successful append to that snapshot.
	discarded := appendSetEntry(original, String("discarded"), setInsertionHash{})
	recovered := appendSetEntry(original, String("recovered"), setInsertionHash{})
	if len(original.Set) != 3 || discarded.Set[3].Text != "discarded" || recovered.Set[3].Text != "recovered" {
		t.Fatal("discarded append corrupted a surviving prefix")
	}
}

func TestNestedReceiverCacheFieldReplacementAndMapKeyRoutes(t *testing.T) {
	for _, route := range []string{"field", "list", "set", "map-value", "map-key"} {
		t.Run(route, func(t *testing.T) {
			machine := New(nil)
			receiver := Object("CacheReader")
			old := List(String("old"))
			receiver.Fields["old"] = old
			unrelated := List()
			machine.collectionMutationSeq++
			machine.recordCollectionMutation(unrelated.Ref)
			if !machine.nestedCollectionsUnchangedSince(receiver, 0, nil) {
				t.Fatal("unrelated mutation marked receiver changed")
			}
			added := List(String("added"))
			var container Value
			switch route {
			case "field":
				container = added
			case "list":
				container = List(added)
			case "set":
				container = Set(added)
			case "map-value":
				container = Map()
				container.Map["child"] = added
			case "map-key":
				container = Map()
				container.MapKeys["child"] = added
			}
			machine.setGraphFieldValue(&receiver, "added", container)
			since := machine.collectionMutationSeq
			machine.collectionMutationSeq++
			machine.recordCollectionMutation(added.Ref)
			if machine.nestedCollectionsUnchangedSince(receiver, since, nil) {
				t.Fatal("replacement hid a newly reachable mutated collection")
			}
			// A current window may cache the graph; an older window must still see
			// the already recorded mutation rather than borrowing the newer verdict.
			if !machine.nestedCollectionsUnchangedSince(receiver, machine.collectionMutationSeq, nil) {
				t.Fatal("current graph treated as stale")
			}
			if machine.nestedCollectionsUnchangedSince(receiver, since, nil) {
				t.Fatal("cached newer window hid older mutation")
			}
		})
	}
}

func TestNestedReceiverCacheRootBackingCloneAndBoundsRoutes(t *testing.T) {
	machine := New(nil)
	other := List()
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(other.Ref)
	root := Object("CacheRoot")
	root.Fields["items"] = List(String("seed"))
	if !machine.nestedCollectionsUnchangedSince(root, 0, nil) {
		t.Fatal("unexpected mutation")
	}
	replacement := root
	replacement.Fields = map[string]Value{"items": List(String("new"))}
	since := machine.collectionMutationSeq
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(replacement.Fields["items"].Ref)
	if machine.nestedCollectionsUnchangedSince(replacement, since, nil) {
		t.Fatal("root ref hid a changed backing")
	}
	clone := machine.CloneRuntime(nil)
	if len(clone.nestedReceiverCache) != 0 {
		t.Fatal("clone inherited a receiver graph")
	}
	for i := 0; i < 100; i++ {
		value := Object(fmt.Sprintf("CacheRoot%d", i))
		value.Fields["items"] = List()
		machine.nestedCollectionsUnchangedSince(value, machine.collectionMutationSeq, nil)
	}
	if len(machine.nestedReceiverCache) > nestedReceiverCacheMaxEntries || machine.nestedReceiverCacheRefs > nestedReceiverCacheMaxRefs {
		t.Fatal("receiver cache exceeded its bounds")
	}
}

func TestNestedReceiverCacheSharedBackingDifferentIdentityRoutes(t *testing.T) {
	for _, route := range []string{"map", "list-slot"} {
		t.Run(route, func(t *testing.T) {
			machine := New(nil)
			other := List()
			machine.collectionMutationSeq++
			machine.recordCollectionMutation(other.Ref)
			receiver := Object("SharedReader")
			nested := List(String("new"))
			if route == "map" {
				container := Map()
				container.Map["old"] = Null
				receiver.Fields["container"] = container
				if !machine.nestedCollectionsUnchangedSince(receiver, 0, nil) {
					t.Fatal("unexpected mutation")
				}
				alias := container
				alias.Ref = newValueRef()
				alias.Map["new"] = nested
				machine.propagateCollectionMutation(container, alias)
			} else {
				container := List(Null)
				receiver.Fields["container"] = container
				if !machine.nestedCollectionsUnchangedSince(receiver, 0, nil) {
					t.Fatal("unexpected mutation")
				}
				alias := container
				alias.Ref = newValueRef()
				machine.setListElement(alias, 0, nested)
			}
			since := machine.collectionMutationSeq
			machine.collectionMutationSeq++
			machine.recordCollectionMutation(nested.Ref)
			if machine.nestedCollectionsUnchangedSince(receiver, since, nil) {
				t.Fatal("shared backing hid newly reachable collection")
			}
		})
	}
}

func TestPrimitiveSetTenThousandAddContainsRoutes(t *testing.T) {
	for _, element := range []string{"String", "Id"} {
		t.Run(element, func(t *testing.T) {
			machine := New(nil)
			set := Set()
			set.Type = "Set<" + element + ">"
			const count = 10000
			keys := make([]Value, count)
			for i := range keys {
				keys[i] = String(fmt.Sprintf("001%012d", i))
				if element == "Id" {
					keys[i].Type = "Id"
				}
			}
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			for _, key := range keys {
				hash, err := machine.captureTypedSetInsertionHash(set.Type, key)
				if err != nil {
					t.Fatal(err)
				}
				index, err := machine.setIndexOfValueWithHash(set, key, hash, &Result{})
				if err != nil || index >= 0 {
					t.Fatal("new key already present")
				}
				set = appendSetEntry(set, key, hash)
			}
			for _, key := range keys {
				found, err := machine.setContainsValue(set, key, &Result{})
				if err != nil || !found {
					t.Fatal("added key absent")
				}
			}
			runtime.ReadMemStats(&after)
			bytes := after.TotalAlloc - before.TotalAlloc
			t.Logf("%d add + contains: bytes=%d elapsed=%s", count, bytes, time.Since(start))
			if bytes > 64<<20 {
				t.Fatalf("linear Set workload allocated %d bytes; want <=64 MiB", bytes)
			}
			runtime.KeepAlive(set)
		})
	}
}

func TestNestedReceiverCacheUntrackedContainerRoute(t *testing.T) {
	machine := New(nil)
	other := List()
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(other.Ref)
	receiver := Object("UntrackedReader")
	container := Value{Kind: ValueMap, Map: map[string]Value{}}
	receiver.Fields["container"] = container
	if !machine.nestedCollectionsUnchangedSince(receiver, 0, nil) {
		t.Fatal("unexpected mutation")
	}
	if len(machine.nestedReceiverCache) != 0 {
		t.Fatal("untracked graph entered identity cache")
	}
}

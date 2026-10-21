package vm

import (
	"fmt"
	"testing"
)

func TestNestedCollectionRefreshFlatReceiverSkipsCallerGraph(t *testing.T) {
	for _, width := range []int{20, 200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			rows := make([]Value, width)
			for i := range rows {
				rows[i] = Object("FlatRow")
				rows[i].Fields["name"] = String("synthetic")
			}
			caller := Object("FlatCaller")
			caller.Fields["rows"] = List(rows...)
			leaf := Object("FlatReceiver")
			leaf.Fields["count"] = Int(7)
			mapping := Map()
			mapping.Map["count"] = Int(7)
			for _, receiver := range []Value{leaf, List(Int(7)), Set(Int(7)), mapping} {
				t.Run(fmt.Sprint(receiver.Kind), func(t *testing.T) {
					machine := New(nil)
					observer := &nestedCollectionRefreshWorkObserver{}
					machine.scopeAliasTraversalObserver = observer
					scope := map[string]Value{"caller": caller, "receiver": receiver}
					machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(receiver), receiver, receiver, true)
					t.Logf("width=%d precheck=%d", width, observer.visits)
					if observer.visits > 8 {
						t.Fatalf("flat receiver scanned unrelated caller graph: visits=%d; want <=8", observer.visits)
					}
				})
			}
		})
	}
}

func TestNestedCollectionRefreshPreservesStaleObjectAliases(t *testing.T) {
	child := Object("SharedChild")
	child.Fields["count"] = Int(1)
	stale := child
	stale.Fields = map[string]Value{"count": Int(0)}
	receiver := Object("SharedReceiver")
	receiver.Fields["child"] = child
	scope := map[string]Value{"stale": stale, "current": child}
	machine := New(nil)
	machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(receiver), receiver, receiver, true)
	if got := scope["stale"].Fields["count"].Int; got != 1 {
		t.Fatalf("stale nested object alias was not refreshed: count=%d; want 1", got)
	}
}

func TestNestedCollectionRefreshReusesBackingStorage(t *testing.T) {
	for _, width := range []int{20, 200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			rows := make([]Value, width)
			for i := range rows {
				rows[i] = Object("SharedRow")
				rows[i].Fields["name"] = String("synthetic")
			}
			receiver := Object("SharedReceiver")
			receiver.Fields["rows"] = List(rows...)
			caller := Object("SharedCaller")
			caller.Fields["receiver"] = receiver
			scope := map[string]Value{"caller": caller}
			machine := New(nil)
			allocations := testing.AllocsPerRun(10, func() {
				if machine.scopeHasNestedCollectionAliasNeedingRefresh(scope, receiver) {
					t.Fatal("current backing unexpectedly needs refresh")
				}
			})
			t.Logf("width=%d allocations=%.0f", width, allocations)
			if allocations > 8 {
				t.Fatalf("reused backing index allocated per graph node: allocations=%.0f; want <=8", allocations)
			}
		})
	}
}

func TestNestedCollectionRefreshReusesNestedBackingStorage(t *testing.T) {
	for _, width := range []int{20, 200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			buckets := make([]Value, width)
			for i := range buckets {
				buckets[i] = Object("SharedBucket")
				buckets[i].Fields["values"] = List(Int(int64(i)))
			}
			receiver := Object("SharedReceiver")
			receiver.Fields["buckets"] = List(buckets...)
			caller := Object("SharedCaller")
			caller.Fields["receiver"] = receiver
			scope := map[string]Value{"caller": caller}
			machine := New(nil)
			allocations := testing.AllocsPerRun(10, func() {
				if machine.scopeHasNestedCollectionAliasNeedingRefresh(scope, receiver) {
					t.Fatal("current nested backing unexpectedly needs refresh")
				}
			})
			t.Logf("width=%d allocations=%.0f", width, allocations)
			if allocations > 8 {
				t.Fatalf("reused nested backings allocated per collection: allocations=%.0f; want <=8", allocations)
			}
		})
	}
}

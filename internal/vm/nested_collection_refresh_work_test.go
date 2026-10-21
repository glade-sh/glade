package vm

import (
	"fmt"
	"testing"
)

type nestedCollectionRefreshWorkObserver struct{ visits uint64 }

func (*nestedCollectionRefreshWorkObserver) scopeAliasRootVisited()         {}
func (*nestedCollectionRefreshWorkObserver) scopeAliasBatchFallback(string) {}
func (observer *nestedCollectionRefreshWorkObserver) nestedCollectionRefreshVisited(visits uint64) {
	observer.visits += visits
}

func TestNestedCollectionRefreshSkipsCurrentCallerGraph(t *testing.T) {
	for _, width := range []int{20, 200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			ledger := Object("BulkLedger")
			buckets := make([]Value, width)
			for i := range buckets {
				bucket := Object("BulkBucket")
				row := Object("Account")
				row.Fields["Name"] = String("synthetic")
				bucket.Fields["rows"] = List(row)
				buckets[i] = bucket
			}
			ledger.Fields["buckets"] = List(buckets...)
			caller := Object("BulkCaller")
			caller.Fields["ledger"] = ledger
			scope := map[string]Value{"caller": caller}
			machine := New(nil)
			observer := &nestedCollectionRefreshWorkObserver{}
			machine.scopeAliasTraversalObserver = observer
			recorder := NewPerfRecorder()
			machine.SetPerfRecorder(recorder)

			machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(ledger), ledger, ledger, true)

			work := recorder.Snapshot().ScopeAlias
			t.Logf("width=%d calls=%d roots=%d recursive=%d precheck=%d", width, work.Calls, work.Roots, work.RecursiveVisits, observer.visits)
			if observer.visits == 0 || observer.visits > uint64(8*width+16) {
				t.Fatalf("precheck visits=%d; want positive work bounded by 8*width+16=%d", observer.visits, 8*width+16)
			}
			if work.Calls != 0 || work.RecursiveVisits != 0 {
				t.Fatalf("already-current nested collections triggered repeated scope scans: calls=%d recursive=%d; want 0", work.Calls, work.RecursiveVisits)
			}
			if got := scope["caller"].Fields["ledger"].Fields["buckets"]; got.Ref != ledger.Fields["buckets"].Ref || len(got.List) != width {
				t.Fatal("unchanged ledger was altered")
			}
		})
	}
}

func TestNestedCollectionRefreshPreservesStaleAliases(t *testing.T) {
	list := List(String("before"))
	listAfter := list
	listAfter.List = []Value{String("after")}
	set := Set(String("before"))
	setAfter := set
	setAfter.Set = []Value{String("after")}
	mapping := Value{Kind: ValueMap, Ref: newValueRef(), Map: map[string]Value{"key": String("before")}}
	mapAfter := mapping
	mapAfter.Map = map[string]Value{"key": String("after")}
	for _, tc := range []struct {
		name          string
		before, after Value
	}{
		{"list", list, listAfter},
		{"set", set, setAfter},
		{"map", mapping, mapAfter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			receiver := Object("CollectionReceiver")
			receiver.Fields["values"] = tc.after
			stale := Object("CollectionCaller")
			stale.Fields["values"] = tc.before
			stale.Fields["self"] = stale
			current := Object("CollectionCaller")
			current.Fields["values"] = tc.after
			// Both current and stale snapshots of the same collection can
			// occur in a caller graph, including behind a cyclic receiver.
			scope := map[string]Value{"current": current, "stale": stale}
			machine := New(nil)
			machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(receiver), receiver, receiver, true)
			got := scope["stale"].Fields["values"]
			if !sameMethodReturnAliasBatchBacking(got, tc.after) {
				t.Fatal("stale collection backing was not refreshed")
			}
		})
	}
}

func TestNestedCollectionRefreshCurrentCycle(t *testing.T) {
	values := List(String("current"))
	receiver := Object("CollectionReceiver")
	receiver.Fields["values"] = values
	caller := Object("CollectionCaller")
	caller.Fields["values"] = values
	caller.Fields["self"] = caller
	scope := map[string]Value{"caller": caller}
	machine := New(nil)
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(receiver), receiver, receiver, true)
	if work := recorder.Snapshot().ScopeAlias; work.Calls != 0 {
		t.Fatalf("current cyclic caller triggered refresh: calls=%d; want 0", work.Calls)
	}
}

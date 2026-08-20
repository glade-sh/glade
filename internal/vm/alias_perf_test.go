package vm

import (
	"sync"
	"testing"
	"time"
)

func TestStaticAliasTopFieldsRankByDurationAndTrackOutcomes(t *testing.T) {
	ResetPerfCounters()
	SetPerfCountersEnabled(true)
	t.Cleanup(ResetPerfCounters)

	recordStaticAliasFieldPerf("Registry.fast", "map", 4, time.Millisecond, true)
	recordStaticAliasFieldPerf("Registry.slow", "object", 2, 3*time.Millisecond, false)
	recordStaticAliasFieldPerf("Registry.fast", "map", 7, 2*time.Millisecond, false)

	fields := SnapshotPerfCounters().StaticAliasTopFields
	if len(fields) != 2 {
		t.Fatalf("field count = %d, want 2: %#v", len(fields), fields)
	}
	if fields[0].Field != "Registry.fast" {
		t.Fatalf("first field = %q, want Registry.fast", fields[0].Field)
	}
	if fields[0].DurationNS != int64(3*time.Millisecond) {
		t.Fatalf("fast duration = %d, want %d", fields[0].DurationNS, int64(3*time.Millisecond))
	}
	if fields[0].Visits != 2 || fields[0].Changed != 1 || fields[0].NoChange != 1 {
		t.Fatalf("fast counters = %#v, want 2 visits, 1 changed, 1 no-change", fields[0])
	}
	if fields[0].MaxChildren != 7 {
		t.Fatalf("fast max children = %d, want 7", fields[0].MaxChildren)
	}
	if fields[1].Field != "Registry.slow" || fields[1].DurationNS != int64(3*time.Millisecond) {
		t.Fatalf("second field = %#v, want Registry.slow with 3ms", fields[1])
	}
}

func TestScopeAliasPerfCapturesCallsRootsVisitsCacheOutcomesAndReplacementTime(t *testing.T) {
	ResetPerfCounters()
	SetPerfCountersEnabled(true)
	t.Cleanup(ResetPerfCounters)

	machine := New(nil)
	target := List(String("before"))
	updated := target
	updated.List = append(updated.List, String("after"))
	containing := List(target)
	containing.Type = "List<List<String>>"
	absent := List(String("absent"))
	absent.Type = "List<String>"

	scope := map[string]Value{"containing": containing, "absent": absent}
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), updated)
	machine.propagateAliasSnapshotToScope(map[string]Value{"absent": absent}, snapshotAlias(target), updated)
	machine.propagateAliasSnapshotToScope(map[string]Value{"absent": absent}, snapshotAlias(target), updated)

	stats := SnapshotPerfCounters().ScopeAlias
	if stats.Calls != 3 || stats.Roots != 4 {
		t.Fatalf("calls/roots = %#v, want 3 calls and 4 roots", stats)
	}
	if stats.RecursiveVisits == 0 || stats.ContainmentCacheMisses == 0 || stats.ContainmentCacheHits == 0 {
		t.Fatalf("containment counters = %#v", stats)
	}
	if stats.ReplacedRoots != 1 || stats.ContainmentNS <= 0 || stats.ReplacementNS <= 0 || stats.DurationNS <= 0 {
		t.Fatalf("replacement/timing counters = %#v", stats)
	}
}

func TestScopeAliasPerfRecordsContainmentCacheClearAndEvictedEntries(t *testing.T) {
	ResetPerfCounters()
	SetPerfCountersEnabled(true)
	t.Cleanup(ResetPerfCounters)

	machine := New(nil)
	machine.aliasContainmentCache = make(map[aliasContainmentCacheKey]uint64, 16385)
	for i := 0; i < 16385; i++ {
		machine.aliasContainmentCache[aliasContainmentCacheKey{ValueRef: uint64(i + 1)}] = 0
	}
	machine.collectionMutationSeq = 1
	machine.recordCollectionMutation(1)

	stats := SnapshotPerfCounters().ScopeAlias
	if stats.ContainmentCacheClears != 1 || stats.ContainmentEntriesEvicted != 16385 {
		t.Fatalf("cache clear counters = %#v", stats)
	}
}

func TestScopeAliasPerfRecordsMutationEpochAdvances(t *testing.T) {
	ResetPerfCounters()
	SetPerfCountersEnabled(true)
	t.Cleanup(ResetPerfCounters)

	machine := New(nil)
	machine.collectionMutationSeq = 7
	machine.recordCollectionMutation(11)
	machine.collectionMutationSeq = 9
	machine.recordCollectionMutation(11)

	stats := SnapshotPerfCounters().ScopeAlias
	if stats.MutationEpochAdvances != 2 || stats.MaxMutationEpoch != 9 {
		t.Fatalf("mutation epoch counters = %#v", stats)
	}
}

func TestScopeAliasPerfDisabledLeavesCountersZero(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)

	machine := New(nil)
	target := List(String("before"))
	updated := target
	updated.List = append(updated.List, String("after"))
	machine.propagateAliasSnapshotToScope(map[string]Value{"target": target}, snapshotAlias(target), updated)
	machine.collectionMutationSeq = 1
	machine.recordCollectionMutation(target.Ref)

	if got := SnapshotPerfCounters().ScopeAlias; got != (ScopeAliasPerfCounters{}) {
		t.Fatalf("disabled scope counters = %#v, want zero", got)
	}
}

func TestConstructedObjectMutationSkipsNestedScopeUntilEscape(t *testing.T) {
	machine := New(nil)
	record, err := machine.constructValue("Account", nil, nil, resultForLookup())
	if err != nil {
		t.Fatal(err)
	}
	machine.Globals["record"] = record
	records := List()
	records.Type = "List<Account>"
	for range 5_000 {
		records.List = append(records.List, Object("Account"))
	}
	machine.Globals["records"] = records
	machine.rememberLocalOnlyCollection(records)

	localRecorder := NewPerfRecorder()
	machine.SetPerfRecorder(localRecorder)
	if err := machine.assignPath(record, []string{"Name"}, String("before")); err != nil {
		t.Fatal(err)
	}
	if visits := localRecorder.Snapshot().ScopeAlias.RecursiveVisits; visits != 0 {
		t.Fatalf("local object mutation recursively visited %d scope values, want 0", visits)
	}

	updated := machine.Globals["record"]
	if _, _, err := machine.callListValueMember("records", records, "add", []Value{updated}, resultForLookup()); err != nil {
		t.Fatal(err)
	}
	escapedRecorder := NewPerfRecorder()
	machine.SetPerfRecorder(escapedRecorder)
	if err := machine.assignPath(updated, []string{"Name"}, String("after")); err != nil {
		t.Fatal(err)
	}
	if visits := escapedRecorder.Snapshot().ScopeAlias.RecursiveVisits; visits == 0 {
		t.Fatal("escaped object mutation did not inspect nested scope aliases")
	}
	got := machine.Globals["records"]
	if name := got.List[len(got.List)-1].Fields["Name"]; name.Kind != ValueString || name.Text != "after" {
		t.Fatalf("escaped nested alias Name = %#v, want after", name)
	}
}

func TestStaticAndScopeAliasDurationsRemainSeparate(t *testing.T) {
	ResetPerfCounters()
	SetPerfCountersEnabled(true)
	t.Cleanup(ResetPerfCounters)

	recordStaticAliasPerf(3*time.Millisecond, false, 1, true)
	machine := New(nil)
	target := List(String("before"))
	updated := target
	updated.List = append(updated.List, String("after"))
	machine.propagateAliasSnapshotToScope(map[string]Value{"target": List(target)}, snapshotAlias(target), updated)

	stats := SnapshotPerfCounters()
	if stats.StaticAlias.DurationNS != int64(3*time.Millisecond) {
		t.Fatalf("static duration = %d, want %d", stats.StaticAlias.DurationNS, int64(3*time.Millisecond))
	}
	if stats.ScopeAlias.DurationNS <= 0 || stats.ScopeAlias.DurationNS == stats.StaticAlias.DurationNS {
		t.Fatalf("static/scope durations mixed: static=%#v scope=%#v", stats.StaticAlias, stats.ScopeAlias)
	}
}

func TestCloneRuntimeCarriesPerfRecorder(t *testing.T) {
	recorder := NewPerfRecorder()
	machine := New(nil)
	machine.SetPerfRecorder(recorder)
	clone := machine.CloneRuntime(nil)

	target := List(String("before"))
	updated := target
	updated.List = append(updated.List, String("after"))
	clone.propagateAliasSnapshotToScope(map[string]Value{"target": List(target)}, snapshotAlias(target), updated)

	if stats := recorder.Snapshot().ScopeAlias; stats.Calls != 1 || stats.ReplacedRoots != 1 {
		t.Fatalf("clone recorder stats = %#v, want one recorded scope replacement", stats)
	}
}

func TestPerfRecordersRemainIndependentDuringConcurrentActivity(t *testing.T) {
	first := NewPerfRecorder()
	second := NewPerfRecorder()
	firstVM := New(nil)
	secondVM := New(nil)
	firstVM.SetPerfRecorder(first)
	secondVM.SetPerfRecorder(second)

	var wg sync.WaitGroup
	wg.Add(2)
	run := func(machine *VM, calls int) {
		defer wg.Done()
		for i := 0; i < calls; i++ {
			target := List(String("before"))
			updated := target
			updated.List = append(updated.List, String("after"))
			machine.propagateAliasSnapshotToScope(map[string]Value{"target": List(target)}, snapshotAlias(target), updated)
		}
	}
	go run(firstVM, 2)
	go run(secondVM, 3)
	wg.Wait()

	if got := first.Snapshot().ScopeAlias.Calls; got != 2 {
		t.Fatalf("first recorder calls = %d, want 2", got)
	}
	if got := second.Snapshot().ScopeAlias.Calls; got != 3 {
		t.Fatalf("second recorder calls = %d, want 3", got)
	}
}

func TestDMLStaticAndScopePerfCountersRemainIndependent(t *testing.T) {
	recorder := NewPerfRecorder()
	machine := New(nil)
	machine.SetPerfRecorder(recorder)

	machine.recordSnapshotDMLRollbackPoint()
	afterDML := recorder.Snapshot()
	if afterDML.DML.RollbackPoints != 1 || afterDML.DML.SnapshotRollbackPoints != 1 {
		t.Fatalf("DML counters = %#v, want one snapshot rollback", afterDML.DML)
	}
	if afterDML.StaticAlias.Calls != 0 || afterDML.ScopeAlias.Calls != 0 {
		t.Fatalf("DML recording changed alias counters: static=%#v scope=%#v", afterDML.StaticAlias, afterDML.ScopeAlias)
	}

	recorder.recordStaticAliasPerf(time.Millisecond, false, 1, true)
	target := List(String("before"))
	updated := target
	updated.List = append(updated.List, String("after"))
	machine.propagateAliasSnapshotToScope(map[string]Value{"target": List(target)}, snapshotAlias(target), updated)
	afterAliases := recorder.Snapshot()
	if afterAliases.StaticAlias.Calls != 1 || afterAliases.ScopeAlias.Calls != 1 {
		t.Fatalf("alias counters = static %#v scope %#v", afterAliases.StaticAlias, afterAliases.ScopeAlias)
	}
	if afterAliases.DML != afterDML.DML {
		t.Fatalf("alias recording changed DML counters: before=%#v after=%#v", afterDML.DML, afterAliases.DML)
	}
}

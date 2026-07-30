package apextest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPerfCountersDisabledByDefault(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	counters := newRunPerfCounters()
	recordRunDuration(time.Millisecond, counters)
	recordCloneReason("ExampleTest", "test", "full-method-isolation", counters)

	stats := snapshotPerfCounters(counters)
	if stats.Enabled {
		t.Fatalf("Enabled = true, want false: %#v", stats)
	}
	if stats.RunDurationMS != 1 {
		t.Fatalf("legacy RunDurationMS = %d, want 1", stats.RunDurationMS)
	}
	if stats.Phases != (RunnerPhasePerfCounters{}) || len(stats.CloneReasons) != 0 || !reflect.DeepEqual(stats.VMPerf, vm.PerfCounters{}) {
		t.Fatalf("disabled detailed snapshot = %#v, want zero phases/reasons/VM", stats)
	}
}

func TestRunPerfCountersClassifyCloneReasonsByClassAndCapability(t *testing.T) {
	counters := newRunPerfCounters(true)
	recordCloneReason("ZuluTest", "test", "full-method-isolation", counters)
	recordCloneReason("AlphaTest", "setup", "org-frozen-shared", counters)
	recordCloneReason("AlphaTest", "setup", "org-frozen-shared", counters)
	recordCloneReason("", "run-base", "org-template", counters)

	got := snapshotPerfCounters(counters).CloneReasons
	want := []PerfCloneReason{
		{Class: "", Reason: "run-base", Capability: "org-template", Count: 1},
		{Class: "AlphaTest", Reason: "setup", Capability: "org-frozen-shared", Count: 2},
		{Class: "ZuluTest", Reason: "test", Capability: "full-method-isolation", Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CloneReasons = %#v, want %#v", got, want)
	}
}

func TestPerfCountersCapturePhaseDurations(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	counters := newRunPerfCounters(true)

	recordSetupDuration(3*time.Millisecond, counters)
	recordRunDuration(5*time.Millisecond, counters)
	recordCloneRuntimeOrgDuration(7*time.Millisecond, counters)
	recordCloneRuntimeMachineDuration(11*time.Millisecond, counters)

	stats := snapshotPerfCounters(counters)
	if stats.SetupDurationMS != 3 {
		t.Fatalf("SetupDurationMS = %d, want 3", stats.SetupDurationMS)
	}
	if stats.RunDurationMS != 5 {
		t.Fatalf("RunDurationMS = %d, want 5", stats.RunDurationMS)
	}
	if stats.CloneRuntimeOrgMS != 7 {
		t.Fatalf("CloneRuntimeOrgMS = %d, want 7", stats.CloneRuntimeOrgMS)
	}
	if stats.CloneRuntimeMachineMS != 11 {
		t.Fatalf("CloneRuntimeMachineMS = %d, want 11", stats.CloneRuntimeMachineMS)
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"setupDurationMs", "runDurationMs", "cloneRuntimeOrgMs", "cloneRuntimeMachineMs"} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("counter JSON missing %s: %s", field, data)
		}
	}
}

func TestPerfCountersCaptureSemanticAndExecutionPhaseAccounting(t *testing.T) {
	counters := newRunPerfCounters(true)
	counters.phases.semanticKeyNS.Add((2 * time.Millisecond).Nanoseconds())
	counters.phases.semanticGateNS.Add((3 * time.Millisecond).Nanoseconds())
	counters.phases.semanticMemoryCacheHits.Add(1)
	counters.phases.semanticDiskCacheHits.Add(0)
	counters.phases.semanticCacheMisses.Add(2)
	counters.phases.methodWindowNS.Add((5 * time.Millisecond).Nanoseconds())
	counters.phases.rollbackNS.Add((7 * time.Millisecond).Nanoseconds())
	counters.phases.teardownNS.Add((11 * time.Millisecond).Nanoseconds())

	got := snapshotPerfCounters(counters).Phases
	if got.SemanticKeyNS != (2*time.Millisecond).Nanoseconds() || got.SemanticGateNS != (3*time.Millisecond).Nanoseconds() {
		t.Fatalf("semantic phase accounting = %#v", got)
	}
	if got.SemanticMemoryCacheHits != 1 || got.SemanticDiskCacheHits != 0 || got.SemanticCacheMisses != 2 {
		t.Fatalf("semantic cache provenance = %#v", got)
	}
	if got.MethodWindowNS != (5*time.Millisecond).Nanoseconds() || got.RollbackNS != (7*time.Millisecond).Nanoseconds() || got.TeardownNS != (11*time.Millisecond).Nanoseconds() {
		t.Fatalf("execution phase accounting = %#v", got)
	}

	data, err := json.Marshal(snapshotPerfCounters(counters))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"semanticKeyNs", "semanticGateNs", "semanticMemoryCacheHits", "semanticDiskCacheHits", "semanticCacheMisses", "methodWindowNs", "rollbackNs", "teardownNs"} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("counter JSON missing %s: %s", field, data)
		}
	}
}

func TestPerfCountersDoNotRecordDetailedPhasesWhenDisabled(t *testing.T) {
	counters := newRunPerfCounters(false)
	recordRunDuration(time.Millisecond, counters)

	if got := snapshotPerfCounters(counters).Phases; got != (RunnerPhasePerfCounters{}) {
		t.Fatalf("disabled detailed phases = %#v, want zero", got)
	}
}

func TestRunPerfCountersRecordSemanticAndWorkerBoundaries(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/PerfBoundaryTest.cls"), `
@isTest
private class PerfBoundaryTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1, PerfCounters: true})
	if summary := run.Summary(); summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	phases := SnapshotPerfCounters().Phases
	if phases.SemanticKeyNS <= 0 || phases.SemanticGateNS <= 0 || phases.SemanticCacheMisses != 1 {
		t.Fatalf("semantic accounting = %#v", phases)
	}
	if phases.SemanticMemoryCacheHits != 0 || phases.SemanticDiskCacheHits != 0 {
		t.Fatalf("semantic cache provenance = %#v", phases)
	}
	if phases.MethodWindowNS <= 0 || phases.RollbackNS <= 0 {
		t.Fatalf("worker boundary accounting = %#v", phases)
	}
	if phases.TeardownNS != 0 {
		t.Fatalf("result assembly was recorded as teardown: %#v", phases)
	}
	classLookup := SnapshotPerfCounters().VMPerf.ClassLookup
	if classLookup.Hits == 0 || classLookup.Misses == 0 {
		t.Fatalf("class lookup accounting = %#v, want runtime hits and misses", classLookup)
	}
}

func TestRunTestPlansDoesNotOpenWorkerWindowForPrecompletedCases(t *testing.T) {
	counters := newRunPerfCounters(true)
	planned := []testCasePlan{{TestCase: TestCase{ClassName: "SkippedTest", MethodName: "skipped"}}}
	results := []testreport.Case{{Status: testreport.StatusUnsupported}}

	runTestPlans(context.Background(), planned, results, nil, nil, nil, Options{Parallelism: 1}, counters)

	if got := snapshotPerfCounters(counters).Phases.MethodWindowNS; got != 0 {
		t.Fatalf("MethodWindowNS = %d, want zero without a dispatched case", got)
	}
}

func TestRunPerfCountersRecordPostJoinDispatcherTeardown(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/PerfTeardownTest.cls"), `
@isTest
private class PerfTeardownTest {
  @testSetup static void setup() { System.assertEquals(1, 1); }
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1, PerfCounters: true})
	if summary := run.Summary(); summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if got := SnapshotPerfCounters().Phases.TeardownNS; got <= 0 {
		t.Fatalf("TeardownNS = %d, want post-join dispatcher cleanup", got)
	}
}

func TestPerfCountersIncludeStorageAndVMStats(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	counters := newRunPerfCounters(true)

	recordStorageCloneRollbackSnapshot(counters)
	counters.captureVMPerf(vm.PerfCounters{
		Enabled: true,
		ClassLookup: vm.ClassLookupPerfCounters{
			Hits:          2,
			Misses:        3,
			Entries:       4,
			Evictions:     5,
			RetainedBytes: 6,
		},
	})

	stats := snapshotPerfCounters(counters)
	if stats.StorageCloneStats.CloneRollbackSnapshotCalls == 0 {
		t.Fatalf("storage clone stats missing rollback snapshot count: %#v", stats.StorageCloneStats)
	}
	if !stats.VMPerf.Enabled {
		t.Fatalf("vm perf counters not marked enabled: %#v", stats.VMPerf)
	}
	if got := stats.VMPerf.ClassLookup; got != (vm.ClassLookupPerfCounters{
		Hits:          2,
		Misses:        3,
		Entries:       4,
		Evictions:     5,
		RetainedBytes: 6,
	}) {
		t.Fatalf("class lookup perf counters = %+v", got)
	}
}

func TestRunPerfCountersAreIndependent(t *testing.T) {
	first := newRunPerfCounters(true)
	second := newRunPerfCounters(true)

	recordSetupDuration(3*time.Millisecond, first)
	recordRunDuration(5*time.Millisecond, first)
	recordCloneRuntimeOrg("FirstTest", "setup", first)
	recordStorageCloneRuntime(first)

	recordSetupDuration(17*time.Millisecond, second)
	recordRunDuration(19*time.Millisecond, second)
	recordCloneRuntimeOrg("SecondTest", "test", second)
	recordStorageCloneRollbackSnapshot(second)

	firstStats := snapshotPerfCounters(first)
	secondStats := snapshotPerfCounters(second)
	if firstStats.SetupDurationMS != 3 || firstStats.RunDurationMS != 5 {
		t.Fatalf("first stats changed: %#v", firstStats)
	}
	if secondStats.SetupDurationMS != 17 || secondStats.RunDurationMS != 19 {
		t.Fatalf("second stats changed: %#v", secondStats)
	}
	if firstStats.StorageCloneStats.CloneRuntimeCalls != 1 || firstStats.StorageCloneStats.CloneRollbackSnapshotCalls != 0 {
		t.Fatalf("first storage stats = %#v, want one runtime clone only", firstStats.StorageCloneStats)
	}
	if secondStats.StorageCloneStats.CloneRuntimeCalls != 1 || secondStats.StorageCloneStats.CloneRollbackSnapshotCalls != 1 {
		t.Fatalf("second storage stats = %#v, want one rollback snapshot", secondStats.StorageCloneStats)
	}
	if firstStats.CloneClasses[0].Class != "FirstTest" {
		t.Fatalf("first clone classes = %#v", firstStats.CloneClasses)
	}
	if secondStats.CloneClasses[0].Class != "SecondTest" {
		t.Fatalf("second clone classes = %#v", secondStats.CloneClasses)
	}
}

func TestRunPerfCountersVMFieldsAreMutationIsolated(t *testing.T) {
	counters := newRunPerfCounters(true)
	counters.captureVMPerf(vm.PerfCounters{
		Enabled: true,
		StaticAliasTopFields: []vm.StaticAliasFieldPerf{
			{Field: "Registry.Values", Visits: 1},
		},
	})

	first := snapshotPerfCounters(counters)
	first.VMPerf.StaticAliasTopFields[0].Field = "Mutated.Values"
	second := snapshotPerfCounters(counters)
	if got := second.VMPerf.StaticAliasTopFields[0].Field; got != "Registry.Values" {
		t.Fatalf("later snapshot field = %q, want isolated Registry.Values", got)
	}
}

func TestPerfCountersLegacyZeroJSONShapeIsPreserved(t *testing.T) {
	data, err := json.Marshal(PerfCounters{})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"cloneRuntimeOrgCalls",
		"journalRollbacks",
		"cloneFallbacks",
		"setupDurationMs",
		"runDurationMs",
		"cloneRuntimeOrgMs",
		"cloneRuntimeMachineMs",
		"storageCloneStats",
		"vmPerf",
	} {
		if _, ok := got[key]; !ok {
			t.Fatalf("legacy zero JSON missing %q: %s", key, data)
		}
	}
	for _, key := range []string{"enabled", "phases", "cloneClasses", "cloneReasons"} {
		if _, ok := got[key]; ok {
			t.Fatalf("zero JSON unexpectedly includes %q: %s", key, data)
		}
	}
	if string(got["storageCloneStats"]) != `{"cloneRuntimeCalls":0,"cloneRollbackSnapshotCalls":0}` {
		t.Fatalf("storage clone JSON shape changed: %s", got["storageCloneStats"])
	}
	var vmShape map[string]json.RawMessage
	if err := json.Unmarshal(got["vmPerf"], &vmShape); err != nil {
		t.Fatal(err)
	}
	if _, ok := vmShape["staticAlias"]; !ok {
		t.Fatalf("legacy VM JSON missing staticAlias: %s", got["vmPerf"])
	}
	if _, ok := vmShape["dml"]; !ok {
		t.Fatalf("legacy VM JSON missing dml: %s", got["vmPerf"])
	}
	if _, ok := vmShape["scopeAlias"]; ok {
		t.Fatalf("zero VM JSON unexpectedly includes new scopeAlias: %s", got["vmPerf"])
	}
}

package apextest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

func TestPerfCountersCapturePhaseDurations(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)

	recordSetupDuration(3 * time.Millisecond)
	recordRunDuration(5 * time.Millisecond)
	recordCloneRuntimeOrgDuration(7 * time.Millisecond)
	recordCloneRuntimeMachineDuration(11 * time.Millisecond)

	stats := SnapshotPerfCounters()
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

func TestPerfCountersIncludeStorageAndVMStats(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)

	recordStorageCloneRollbackSnapshot()
	vm.SetPerfCountersEnabled(true)

	stats := SnapshotPerfCounters()
	if stats.StorageCloneStats.CloneRollbackSnapshotCalls == 0 {
		t.Fatalf("storage clone stats missing rollback snapshot count: %#v", stats.StorageCloneStats)
	}
	if !stats.VMPerf.Enabled {
		t.Fatalf("vm perf counters not marked enabled: %#v", stats.VMPerf)
	}
}

func TestRunPerfCountersAreIndependent(t *testing.T) {
	first := newRunPerfCounters()
	second := newRunPerfCounters()

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

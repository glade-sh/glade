package apextest

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type PerfCounters struct {
	CloneRuntimeOrgCalls  uint64             `json:"cloneRuntimeOrgCalls"`
	JournalRollbacks      uint64             `json:"journalRollbacks"`
	CloneFallbacks        uint64             `json:"cloneFallbacks"`
	SetupDurationMS       int64              `json:"setupDurationMs"`
	RunDurationMS         int64              `json:"runDurationMs"`
	CloneRuntimeOrgMS     int64              `json:"cloneRuntimeOrgMs"`
	CloneRuntimeMachineMS int64              `json:"cloneRuntimeMachineMs"`
	StorageCloneStats     storage.CloneStats `json:"storageCloneStats"`
	VMPerf                vm.PerfCounters    `json:"vmPerf,omitempty"`
	CloneClasses          []PerfCloneClass   `json:"cloneClasses,omitempty"`
}

type PerfCloneClass struct {
	Class       string `json:"class"`
	SetupClones uint64 `json:"setupClones,omitempty"`
	TestClones  uint64 `json:"testClones,omitempty"`
}

type runPerfCounters struct {
	cloneRuntimeOrg       atomic.Uint64
	journalRollbacks      atomic.Uint64
	cloneFallbacks        atomic.Uint64
	setupDurationMS       atomic.Int64
	runDurationMS         atomic.Int64
	cloneRuntimeOrgMS     atomic.Int64
	cloneRuntimeMachineMS atomic.Int64
	storageCloneRuntime   atomic.Uint64
	storageCloneRollback  atomic.Uint64
	mu                    sync.Mutex
	classes               map[string]*PerfCloneClass
}

var compatibilityPerfCounters atomic.Pointer[runPerfCounters]

func newRunPerfCounters() *runPerfCounters {
	return &runPerfCounters{}
}

func currentPerfCounters() *runPerfCounters {
	if counters := compatibilityPerfCounters.Load(); counters != nil {
		return counters
	}
	counters := newRunPerfCounters()
	if compatibilityPerfCounters.CompareAndSwap(nil, counters) {
		return counters
	}
	return compatibilityPerfCounters.Load()
}

func publishPerfCounters(counters *runPerfCounters) {
	if counters == nil {
		return
	}
	compatibilityPerfCounters.Store(counters)
}

func ResetPerfCounters() {
	storage.ResetCloneStats()
	vm.ResetPerfCounters()
	compatibilityPerfCounters.Store(newRunPerfCounters())
}

func SnapshotPerfCounters() PerfCounters {
	return snapshotPerfCounters(currentPerfCounters())
}

func snapshotPerfCounters(perfCounters *runPerfCounters) PerfCounters {
	if perfCounters == nil {
		return PerfCounters{}
	}
	perfCounters.mu.Lock()
	defer perfCounters.mu.Unlock()
	out := PerfCounters{
		CloneRuntimeOrgCalls:  perfCounters.cloneRuntimeOrg.Load(),
		JournalRollbacks:      perfCounters.journalRollbacks.Load(),
		CloneFallbacks:        perfCounters.cloneFallbacks.Load(),
		SetupDurationMS:       perfCounters.setupDurationMS.Load(),
		RunDurationMS:         perfCounters.runDurationMS.Load(),
		CloneRuntimeOrgMS:     perfCounters.cloneRuntimeOrgMS.Load(),
		CloneRuntimeMachineMS: perfCounters.cloneRuntimeMachineMS.Load(),
		StorageCloneStats: storage.CloneStats{
			CloneRuntimeCalls:          perfCounters.storageCloneRuntime.Load(),
			CloneRollbackSnapshotCalls: perfCounters.storageCloneRollback.Load(),
		},
		VMPerf: vm.SnapshotPerfCounters(),
	}
	if len(perfCounters.classes) > 0 {
		out.CloneClasses = make([]PerfCloneClass, 0, len(perfCounters.classes))
		for _, entry := range perfCounters.classes {
			out.CloneClasses = append(out.CloneClasses, *entry)
		}
		sort.Slice(out.CloneClasses, func(i, j int) bool {
			left := out.CloneClasses[i].SetupClones + out.CloneClasses[i].TestClones
			right := out.CloneClasses[j].SetupClones + out.CloneClasses[j].TestClones
			if left == right {
				return out.CloneClasses[i].Class < out.CloneClasses[j].Class
			}
			return left > right
		})
	}
	return out
}

func perfCounterFor(counters []*runPerfCounters) *runPerfCounters {
	if len(counters) > 0 && counters[0] != nil {
		return counters[0]
	}
	return currentPerfCounters()
}

func recordJournalRollback(counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.journalRollbacks.Add(1)
}

func recordCloneFallback(counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.cloneFallbacks.Add(1)
}

func recordSetupDuration(d time.Duration, counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.setupDurationMS.Add(d.Milliseconds())
}

func recordRunDuration(d time.Duration, counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.runDurationMS.Add(d.Milliseconds())
}

func recordCloneRuntimeOrgDuration(d time.Duration, counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.cloneRuntimeOrgMS.Add(d.Milliseconds())
}

func recordCloneRuntimeMachineDuration(d time.Duration, counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.cloneRuntimeMachineMS.Add(d.Milliseconds())
}

func recordCloneRuntimeOrg(className, phase string, counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.cloneRuntimeOrg.Add(1)
	if className == "" {
		return
	}
	perfCounters.mu.Lock()
	defer perfCounters.mu.Unlock()
	if perfCounters.classes == nil {
		perfCounters.classes = make(map[string]*PerfCloneClass)
	}
	entry := perfCounters.classes[className]
	if entry == nil {
		entry = &PerfCloneClass{Class: className}
		perfCounters.classes[className] = entry
	}
	switch phase {
	case "setup":
		entry.SetupClones++
	case "test":
		entry.TestClones++
	}
}

func recordStorageCloneRuntime(counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.storageCloneRuntime.Add(1)
}

func recordStorageCloneRollbackSnapshot(counters ...*runPerfCounters) {
	perfCounters := perfCounterFor(counters)
	perfCounters.storageCloneRollback.Add(1)
	perfCounters.storageCloneRuntime.Add(1)
}

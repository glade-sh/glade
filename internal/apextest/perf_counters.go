package apextest

import (
	"sort"
	"sync"
	"sync/atomic"
)

type PerfCounters struct {
	CloneRuntimeOrgCalls uint64           `json:"cloneRuntimeOrgCalls"`
	JournalRollbacks     uint64           `json:"journalRollbacks"`
	CloneFallbacks       uint64           `json:"cloneFallbacks"`
	CloneClasses         []PerfCloneClass `json:"cloneClasses,omitempty"`
}

type PerfCloneClass struct {
	Class       string `json:"class"`
	SetupClones uint64 `json:"setupClones,omitempty"`
	TestClones  uint64 `json:"testClones,omitempty"`
}

var perfCounters struct {
	cloneRuntimeOrg  atomic.Uint64
	journalRollbacks atomic.Uint64
	cloneFallbacks   atomic.Uint64
	mu               sync.Mutex
	classes          map[string]*PerfCloneClass
}

func ResetPerfCounters() {
	perfCounters.cloneRuntimeOrg.Store(0)
	perfCounters.journalRollbacks.Store(0)
	perfCounters.cloneFallbacks.Store(0)
	perfCounters.mu.Lock()
	perfCounters.classes = nil
	perfCounters.mu.Unlock()
}

func SnapshotPerfCounters() PerfCounters {
	perfCounters.mu.Lock()
	defer perfCounters.mu.Unlock()
	out := PerfCounters{
		CloneRuntimeOrgCalls: perfCounters.cloneRuntimeOrg.Load(),
		JournalRollbacks:     perfCounters.journalRollbacks.Load(),
		CloneFallbacks:       perfCounters.cloneFallbacks.Load(),
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

func recordJournalRollback() {
	perfCounters.journalRollbacks.Add(1)
}

func recordCloneFallback() {
	perfCounters.cloneFallbacks.Add(1)
}

func recordCloneRuntimeOrg(className, phase string) {
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

package apextest

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type PerfCounters struct {
	Enabled                          bool                    `json:"enabled,omitempty"`
	Phases                           RunnerPhasePerfCounters `json:"phases,omitzero"`
	CloneRuntimeOrgCalls             uint64                  `json:"cloneRuntimeOrgCalls"`
	JournalRollbacks                 uint64                  `json:"journalRollbacks"`
	CloneFallbacks                   uint64                  `json:"cloneFallbacks"`
	SetupDurationMS                  int64                   `json:"setupDurationMs"`
	RunDurationMS                    int64                   `json:"runDurationMs"`
	CloneRuntimeOrgMS                int64                   `json:"cloneRuntimeOrgMs"`
	CloneRuntimeMachineMS            int64                   `json:"cloneRuntimeMachineMs"`
	RuntimeTransitionLeaders         uint64                  `json:"runtimeTransitionLeaders,omitempty"`
	RuntimeTransitionWaiters         uint64                  `json:"runtimeTransitionWaiters,omitempty"`
	RuntimeTransitionSharedFallbacks uint64                  `json:"runtimeTransitionSharedFallbacks,omitempty"`
	RuntimeTransitionCancellations   uint64                  `json:"runtimeTransitionCancellations,omitempty"`
	RuntimeTransitionSavedBytes      uint64                  `json:"runtimeTransitionSavedBytes,omitempty"`
	RuntimeFingerprintBytes          uint64                  `json:"runtimeFingerprintBytes,omitempty"`
	StorageCloneStats                storage.CloneStats      `json:"storageCloneStats"`
	VMPerf                           vm.PerfCounters         `json:"vmPerf,omitempty"`
	SemanticCache                    SemanticCachePerf       `json:"semanticCache,omitzero"`
	CloneClasses                     []PerfCloneClass        `json:"cloneClasses,omitempty"`
	CloneReasons                     []PerfCloneReason       `json:"cloneReasons,omitempty"`
}

type SemanticCachePerf struct {
	Identity      semanticcache.Identity   `json:"identity"`
	Source        semanticcache.Source     `json:"source,omitempty"`
	Waited        bool                     `json:"waited,omitempty"`
	MissReason    semanticcache.MissReason `json:"missReason,omitempty"`
	RetainedBytes int64                    `json:"retainedBytes,omitempty"`
	Evictions     uint64                   `json:"evictions,omitempty"`
}

type RunnerPhasePerfCounters struct {
	ProjectLoadNS           int64  `json:"projectLoadNs,omitempty"`
	SchemaLoadNS            int64  `json:"schemaLoadNs,omitempty"`
	IndexBuildNS            int64  `json:"indexBuildNs,omitempty"`
	DiscoverNS              int64  `json:"discoverNs,omitempty"`
	SemanticKeyNS           int64  `json:"semanticKeyNs,omitempty"`
	SemanticGateNS          int64  `json:"semanticGateNs,omitempty"`
	SemanticMemoryCacheHits uint64 `json:"semanticMemoryCacheHits,omitempty"`
	SemanticDiskCacheHits   uint64 `json:"semanticDiskCacheHits"`
	SemanticCacheMisses     uint64 `json:"semanticCacheMisses,omitempty"`
	SemanticBuilds          uint64 `json:"semanticBuilds,omitempty"`
	SemanticAnalyses        uint64 `json:"semanticAnalyses,omitempty"`
	SemanticWaits           uint64 `json:"semanticWaits,omitempty"`
	SemanticErrors          uint64 `json:"semanticErrors,omitempty"`
	SemanticEvictions       uint64 `json:"semanticEvictions,omitempty"`
	SemanticRetainedBytes   int64  `json:"semanticRetainedBytes,omitempty"`
	RuntimeKeyNS            int64  `json:"runtimeKeyNs,omitempty"`
	CacheValidateNS         int64  `json:"cacheValidateNs,omitempty"`
	CacheDecodeNS           int64  `json:"cacheDecodeNs,omitempty"`
	CacheEncodeNS           int64  `json:"cacheEncodeNs,omitempty"`
	MemoryCacheHits         uint64 `json:"memoryCacheHits,omitempty"`
	DiskCacheHits           uint64 `json:"diskCacheHits,omitempty"`
	CacheMisses             uint64 `json:"cacheMisses,omitempty"`
	OrgBuildNS              int64  `json:"orgBuildNs,omitempty"`
	ProjectCompileNS        int64  `json:"projectCompileNs,omitempty"`
	TestCompileNS           int64  `json:"testCompileNs,omitempty"`
	ClassSetupNS            int64  `json:"classSetupNs,omitempty"`
	MethodRunNS             int64  `json:"methodRunNs,omitempty"`
	MethodWindowNS          int64  `json:"methodWindowNs,omitempty"`
	RollbackNS              int64  `json:"rollbackNs,omitempty"`
	TeardownNS              int64  `json:"teardownNs,omitempty"`
	HistoryApplyNS          int64  `json:"historyApplyNs,omitempty"`
	ReportAssemblyNS        int64  `json:"reportAssemblyNs,omitempty"`
}

type PerfCloneClass struct {
	Class       string `json:"class"`
	SetupClones uint64 `json:"setupClones,omitempty"`
	TestClones  uint64 `json:"testClones,omitempty"`
}

type PerfCloneReason struct {
	Class      string `json:"class,omitempty"`
	Reason     string `json:"reason"`
	Capability string `json:"capability"`
	Count      uint64 `json:"count"`
}

type cloneReasonKey struct {
	class      string
	reason     string
	capability string
}

type runPhaseCounters struct {
	projectLoadNS           atomic.Int64
	schemaLoadNS            atomic.Int64
	indexBuildNS            atomic.Int64
	discoverNS              atomic.Int64
	semanticKeyNS           atomic.Int64
	semanticGateNS          atomic.Int64
	semanticMemoryCacheHits atomic.Uint64
	semanticDiskCacheHits   atomic.Uint64
	semanticCacheMisses     atomic.Uint64
	semanticBuilds          atomic.Uint64
	semanticAnalyses        atomic.Uint64
	semanticWaits           atomic.Uint64
	semanticErrors          atomic.Uint64
	semanticEvictions       atomic.Uint64
	semanticRetainedBytes   atomic.Int64
	runtimeKeyNS            atomic.Int64
	cacheValidateNS         atomic.Int64
	cacheDecodeNS           atomic.Int64
	cacheEncodeNS           atomic.Int64
	memoryCacheHits         atomic.Uint64
	diskCacheHits           atomic.Uint64
	cacheMisses             atomic.Uint64
	orgBuildNS              atomic.Int64
	projectCompileNS        atomic.Int64
	testCompileNS           atomic.Int64
	classSetupNS            atomic.Int64
	methodRunNS             atomic.Int64
	methodWindowNS          atomic.Int64
	rollbackNS              atomic.Int64
	teardownNS              atomic.Int64
	historyApplyNS          atomic.Int64
	reportAssemblyNS        atomic.Int64
}

type runPerfCounters struct {
	enabled                          bool
	cloneRuntimeOrg                  atomic.Uint64
	journalRollbacks                 atomic.Uint64
	cloneFallbacks                   atomic.Uint64
	setupDurationMS                  atomic.Int64
	runDurationMS                    atomic.Int64
	cloneRuntimeOrgMS                atomic.Int64
	cloneRuntimeMachineMS            atomic.Int64
	runtimeTransitionLeaders         atomic.Uint64
	runtimeTransitionWaiters         atomic.Uint64
	runtimeTransitionSharedFallbacks atomic.Uint64
	runtimeTransitionCancellations   atomic.Uint64
	runtimeTransitionSavedBytes      atomic.Uint64
	runtimeFingerprintBytes          atomic.Uint64
	storageCloneRuntime              atomic.Uint64
	storageCloneRollback             atomic.Uint64
	phases                           runPhaseCounters
	mu                               sync.Mutex
	vmPerf                           vm.PerfCounters
	semanticCache                    SemanticCachePerf
	classes                          map[string]*PerfCloneClass
	cloneReasons                     map[cloneReasonKey]uint64
}

var compatibilityPerfCounters atomic.Pointer[runPerfCounters]

func newRunPerfCounters(enabled ...bool) *runPerfCounters {
	return &runPerfCounters{enabled: len(enabled) > 0 && enabled[0]}
}

func currentPerfCounters() *runPerfCounters {
	if counters := compatibilityPerfCounters.Load(); counters != nil {
		return counters
	}
	counters := newRunPerfCounters(false)
	if compatibilityPerfCounters.CompareAndSwap(nil, counters) {
		return counters
	}
	return compatibilityPerfCounters.Load()
}

func publishPerfCounters(counters *runPerfCounters) {
	if counters != nil {
		compatibilityPerfCounters.Store(counters)
	}
}

func ResetPerfCounters() {
	storage.ResetCloneStats()
	vm.ResetPerfCounters()
	compatibilityPerfCounters.Store(newRunPerfCounters(false))
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
		Enabled:                          perfCounters.enabled,
		CloneRuntimeOrgCalls:             perfCounters.cloneRuntimeOrg.Load(),
		JournalRollbacks:                 perfCounters.journalRollbacks.Load(),
		CloneFallbacks:                   perfCounters.cloneFallbacks.Load(),
		SetupDurationMS:                  perfCounters.setupDurationMS.Load(),
		RunDurationMS:                    perfCounters.runDurationMS.Load(),
		CloneRuntimeOrgMS:                perfCounters.cloneRuntimeOrgMS.Load(),
		CloneRuntimeMachineMS:            perfCounters.cloneRuntimeMachineMS.Load(),
		RuntimeTransitionLeaders:         perfCounters.runtimeTransitionLeaders.Load(),
		RuntimeTransitionWaiters:         perfCounters.runtimeTransitionWaiters.Load(),
		RuntimeTransitionSharedFallbacks: perfCounters.runtimeTransitionSharedFallbacks.Load(),
		RuntimeTransitionCancellations:   perfCounters.runtimeTransitionCancellations.Load(),
		RuntimeTransitionSavedBytes:      perfCounters.runtimeTransitionSavedBytes.Load(),
		RuntimeFingerprintBytes:          perfCounters.runtimeFingerprintBytes.Load(),
		StorageCloneStats: storage.CloneStats{
			CloneRuntimeCalls:          perfCounters.storageCloneRuntime.Load(),
			CloneRollbackSnapshotCalls: perfCounters.storageCloneRollback.Load(),
		},
		SemanticCache: perfCounters.semanticCache,
	}
	if perfCounters.enabled {
		out.Phases = perfCounters.snapshotPhases()
		out.VMPerf = cloneVMPerfCounters(perfCounters.vmPerf)
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
	if perfCounters.enabled && len(perfCounters.cloneReasons) > 0 {
		out.CloneReasons = make([]PerfCloneReason, 0, len(perfCounters.cloneReasons))
		for key, count := range perfCounters.cloneReasons {
			out.CloneReasons = append(out.CloneReasons, PerfCloneReason{Class: key.class, Reason: key.reason, Capability: key.capability, Count: count})
		}
		sort.Slice(out.CloneReasons, func(i, j int) bool {
			if out.CloneReasons[i].Class != out.CloneReasons[j].Class {
				return out.CloneReasons[i].Class < out.CloneReasons[j].Class
			}
			if out.CloneReasons[i].Reason != out.CloneReasons[j].Reason {
				return out.CloneReasons[i].Reason < out.CloneReasons[j].Reason
			}
			return out.CloneReasons[i].Capability < out.CloneReasons[j].Capability
		})
	}
	return out
}

func (c *runPerfCounters) captureVMPerf(stats vm.PerfCounters) {
	if c == nil || !c.enabled {
		return
	}
	c.mu.Lock()
	c.vmPerf = cloneVMPerfCounters(stats)
	c.mu.Unlock()
}

func cloneVMPerfCounters(stats vm.PerfCounters) vm.PerfCounters {
	out := stats
	out.StaticAliasTopFields = append([]vm.StaticAliasFieldPerf(nil), stats.StaticAliasTopFields...)
	return out
}

func (c *runPerfCounters) snapshotPhases() RunnerPhasePerfCounters {
	return RunnerPhasePerfCounters{
		ProjectLoadNS:           c.phases.projectLoadNS.Load(),
		SchemaLoadNS:            c.phases.schemaLoadNS.Load(),
		IndexBuildNS:            c.phases.indexBuildNS.Load(),
		DiscoverNS:              c.phases.discoverNS.Load(),
		SemanticKeyNS:           c.phases.semanticKeyNS.Load(),
		SemanticGateNS:          c.phases.semanticGateNS.Load(),
		SemanticMemoryCacheHits: c.phases.semanticMemoryCacheHits.Load(),
		SemanticDiskCacheHits:   c.phases.semanticDiskCacheHits.Load(),
		SemanticCacheMisses:     c.phases.semanticCacheMisses.Load(),
		SemanticBuilds:          c.phases.semanticBuilds.Load(),
		SemanticAnalyses:        c.phases.semanticAnalyses.Load(),
		SemanticWaits:           c.phases.semanticWaits.Load(),
		SemanticErrors:          c.phases.semanticErrors.Load(),
		SemanticEvictions:       c.phases.semanticEvictions.Load(),
		SemanticRetainedBytes:   c.phases.semanticRetainedBytes.Load(),
		RuntimeKeyNS:            c.phases.runtimeKeyNS.Load(),
		CacheValidateNS:         c.phases.cacheValidateNS.Load(),
		CacheDecodeNS:           c.phases.cacheDecodeNS.Load(),
		CacheEncodeNS:           c.phases.cacheEncodeNS.Load(),
		MemoryCacheHits:         c.phases.memoryCacheHits.Load(),
		DiskCacheHits:           c.phases.diskCacheHits.Load(),
		CacheMisses:             c.phases.cacheMisses.Load(),
		OrgBuildNS:              c.phases.orgBuildNS.Load(),
		ProjectCompileNS:        c.phases.projectCompileNS.Load(),
		TestCompileNS:           c.phases.testCompileNS.Load(),
		ClassSetupNS:            c.phases.classSetupNS.Load(),
		MethodRunNS:             c.phases.methodRunNS.Load(),
		MethodWindowNS:          c.phases.methodWindowNS.Load(),
		RollbackNS:              c.phases.rollbackNS.Load(),
		TeardownNS:              c.phases.teardownNS.Load(),
		HistoryApplyNS:          c.phases.historyApplyNS.Load(),
		ReportAssemblyNS:        c.phases.reportAssemblyNS.Load(),
	}
}

func (c *runPerfCounters) recordSemanticCache(identity semanticcache.Identity, access semanticcache.Access) {
	if c == nil || !c.enabled {
		return
	}
	c.phases.semanticRetainedBytes.Store(access.RetainedBytes)
	c.phases.semanticEvictions.Store(access.Evictions)
	if access.Waited {
		c.phases.semanticWaits.Add(1)
	}
	c.mu.Lock()
	c.semanticCache = SemanticCachePerf{
		Identity:      identity,
		Source:        access.Source,
		Waited:        access.Waited,
		MissReason:    access.DiskMissReason,
		RetainedBytes: access.RetainedBytes,
		Evictions:     access.Evictions,
	}
	c.mu.Unlock()
}

func perfCounterFor(counters []*runPerfCounters) *runPerfCounters {
	if len(counters) > 0 && counters[0] != nil {
		return counters[0]
	}
	return currentPerfCounters()
}

func recordJournalRollback(counters ...*runPerfCounters) {
	perfCounterFor(counters).journalRollbacks.Add(1)
}

func recordCloneFallback(counters ...*runPerfCounters) {
	perfCounterFor(counters).cloneFallbacks.Add(1)
}

func recordSetupDuration(d time.Duration, counters ...*runPerfCounters) {
	c := perfCounterFor(counters)
	c.setupDurationMS.Add(d.Milliseconds())
	if c.enabled {
		c.phases.classSetupNS.Add(d.Nanoseconds())
	}
}

func recordRunDuration(d time.Duration, counters ...*runPerfCounters) {
	c := perfCounterFor(counters)
	c.runDurationMS.Add(d.Milliseconds())
	if c.enabled {
		c.phases.methodRunNS.Add(d.Nanoseconds())
	}
}

func startMethodWindow(counters ...*runPerfCounters) time.Time {
	c := perfCounterFor(counters)
	if !c.enabled {
		return time.Time{}
	}
	return time.Now()
}

func recordMethodWindow(started time.Time, counters ...*runPerfCounters) {
	c := perfCounterFor(counters)
	if c.enabled && !started.IsZero() {
		c.phases.methodWindowNS.Add(time.Since(started).Nanoseconds())
	}
}

func recordCloneRuntimeOrgDuration(d time.Duration, counters ...*runPerfCounters) {
	perfCounterFor(counters).cloneRuntimeOrgMS.Add(d.Milliseconds())
}

func recordCloneRuntimeMachineDuration(d time.Duration, counters ...*runPerfCounters) {
	perfCounterFor(counters).cloneRuntimeMachineMS.Add(d.Milliseconds())
}

func recordCloneRuntimeOrg(className, phase string, counters ...*runPerfCounters) {
	c := perfCounterFor(counters)
	c.cloneRuntimeOrg.Add(1)
	if className == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.classes == nil {
		c.classes = make(map[string]*PerfCloneClass)
	}
	entry := c.classes[className]
	if entry == nil {
		entry = &PerfCloneClass{Class: className}
		c.classes[className] = entry
	}
	switch phase {
	case "setup":
		entry.SetupClones++
	case "test":
		entry.TestClones++
	}
}

func recordCloneReason(className, reason, capability string, counters ...*runPerfCounters) {
	c := perfCounterFor(counters)
	if !c.enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cloneReasons == nil {
		c.cloneReasons = make(map[cloneReasonKey]uint64)
	}
	c.cloneReasons[cloneReasonKey{class: className, reason: reason, capability: capability}]++
}

func recordStorageCloneRuntime(counters ...*runPerfCounters) {
	perfCounterFor(counters).storageCloneRuntime.Add(1)
}

func recordStorageCloneRollbackSnapshot(counters ...*runPerfCounters) {
	c := perfCounterFor(counters)
	c.storageCloneRollback.Add(1)
	c.storageCloneRuntime.Add(1)
}

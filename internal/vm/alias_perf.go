package vm

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type PerfCounters struct {
	Enabled              bool                    `json:"enabled,omitempty"`
	StaticAlias          StaticAliasPerfCounters `json:"staticAlias,omitempty"`
	ScopeAlias           ScopeAliasPerfCounters  `json:"scopeAlias,omitzero"`
	StaticAliasTopFields []StaticAliasFieldPerf  `json:"staticAliasTopFields,omitempty"`
	DML                  DMLPerfCounters         `json:"dml,omitempty"`
}

type ScopeAliasPerfCounters struct {
	Calls                     uint64 `json:"calls,omitempty"`
	Roots                     uint64 `json:"roots,omitempty"`
	RecursiveVisits           uint64 `json:"recursiveVisits,omitempty"`
	ContainmentCacheHits      uint64 `json:"containmentCacheHits,omitempty"`
	ContainmentCacheMisses    uint64 `json:"containmentCacheMisses,omitempty"`
	ContainmentCacheClears    uint64 `json:"containmentCacheClears,omitempty"`
	ContainmentEntriesEvicted uint64 `json:"containmentEntriesEvicted,omitempty"`
	MutationEpochAdvances     uint64 `json:"mutationEpochAdvances,omitempty"`
	MaxMutationEpoch          uint64 `json:"maxMutationEpoch,omitempty"`
	ReplacedRoots             uint64 `json:"replacedRoots,omitempty"`
	ContainmentNS             int64  `json:"containmentNs,omitempty"`
	ReplacementNS             int64  `json:"replacementNs,omitempty"`
	DurationNS                int64  `json:"durationNs,omitempty"`
}

type StaticAliasPerfCounters struct {
	Calls           uint64 `json:"calls,omitempty"`
	RefMisses       uint64 `json:"refMisses,omitempty"`
	LocationVisits  uint64 `json:"locationVisits,omitempty"`
	ChildHintHits   uint64 `json:"childHintHits,omitempty"`
	DirectChildHits uint64 `json:"directChildHits,omitempty"`
	Changed         uint64 `json:"changed,omitempty"`
	NoChange        uint64 `json:"noChange,omitempty"`
	DurationNS      int64  `json:"durationNs,omitempty"`
	CollectCalls    uint64 `json:"collectCalls,omitempty"`
	CollectNS       int64  `json:"collectNs,omitempty"`
}

type StaticAliasFieldPerf struct {
	Field       string `json:"field"`
	Visits      uint64 `json:"visits"`
	Changed     uint64 `json:"changed,omitempty"`
	NoChange    uint64 `json:"noChange,omitempty"`
	DurationNS  int64  `json:"durationNs,omitempty"`
	Kind        string `json:"kind,omitempty"`
	MaxChildren int    `json:"maxChildren,omitempty"`
}

type DMLPerfCounters struct {
	RollbackPoints         uint64 `json:"rollbackPoints,omitempty"`
	SnapshotRollbackPoints uint64 `json:"snapshotRollbackPoints,omitempty"`
	JournalRollbackPoints  uint64 `json:"journalRollbackPoints,omitempty"`
	TemporaryJournalPoints uint64 `json:"temporaryJournalPoints,omitempty"`
}

// PerfRecorder is an opaque, concurrency-safe aggregation session. A runtime
// clone shares its recorder pointer with its source VM so one run owns all VM
// measurements produced by its worker clones.
type PerfRecorder struct {
	staticAlias struct {
		calls           atomic.Uint64
		refMisses       atomic.Uint64
		locationVisits  atomic.Uint64
		childHintHits   atomic.Uint64
		directChildHits atomic.Uint64
		changed         atomic.Uint64
		noChange        atomic.Uint64
		durationNS      atomic.Int64
		collectCalls    atomic.Uint64
		collectNS       atomic.Int64
		mu              sync.Mutex
		fields          map[string]*StaticAliasFieldPerf
	}
	scopeAlias struct {
		calls                     atomic.Uint64
		roots                     atomic.Uint64
		recursiveVisits           atomic.Uint64
		containmentCacheHits      atomic.Uint64
		containmentCacheMisses    atomic.Uint64
		containmentCacheClears    atomic.Uint64
		containmentEntriesEvicted atomic.Uint64
		mutationEpochAdvances     atomic.Uint64
		maxMutationEpoch          atomic.Uint64
		replacedRoots             atomic.Uint64
		containmentNS             atomic.Int64
		replacementNS             atomic.Int64
		durationNS                atomic.Int64
	}
	dml struct {
		rollbackPoints         atomic.Uint64
		snapshotRollbackPoints atomic.Uint64
		journalRollbackPoints  atomic.Uint64
		temporaryJournalPoints atomic.Uint64
	}
}

var compatibilityPerfRecorder atomic.Pointer[PerfRecorder]

func NewPerfRecorder() *PerfRecorder {
	return &PerfRecorder{}
}

func (recorder *PerfRecorder) Snapshot() PerfCounters {
	if recorder == nil {
		return PerfCounters{}
	}
	return PerfCounters{
		Enabled:              true,
		StaticAlias:          recorder.snapshotStaticAlias(),
		ScopeAlias:           recorder.snapshotScopeAlias(),
		StaticAliasTopFields: recorder.snapshotStaticAliasTopFields(20),
		DML:                  recorder.snapshotDML(),
	}
}

// SetPerfCountersEnabled preserves the package-level compatibility surface for
// direct VM callers. New VMs capture the current compatibility recorder.
func SetPerfCountersEnabled(enabled bool) {
	if !enabled {
		compatibilityPerfRecorder.Store(nil)
		return
	}
	compatibilityPerfRecorder.Store(NewPerfRecorder())
}

func ResetPerfCounters() {
	compatibilityPerfRecorder.Store(nil)
}

func SnapshotPerfCounters() PerfCounters {
	return compatibilityPerfRecorder.Load().Snapshot()
}

func (recorder *PerfRecorder) snapshotScopeAlias() ScopeAliasPerfCounters {
	return ScopeAliasPerfCounters{
		Calls:                     recorder.scopeAlias.calls.Load(),
		Roots:                     recorder.scopeAlias.roots.Load(),
		RecursiveVisits:           recorder.scopeAlias.recursiveVisits.Load(),
		ContainmentCacheHits:      recorder.scopeAlias.containmentCacheHits.Load(),
		ContainmentCacheMisses:    recorder.scopeAlias.containmentCacheMisses.Load(),
		ContainmentCacheClears:    recorder.scopeAlias.containmentCacheClears.Load(),
		ContainmentEntriesEvicted: recorder.scopeAlias.containmentEntriesEvicted.Load(),
		MutationEpochAdvances:     recorder.scopeAlias.mutationEpochAdvances.Load(),
		MaxMutationEpoch:          recorder.scopeAlias.maxMutationEpoch.Load(),
		ReplacedRoots:             recorder.scopeAlias.replacedRoots.Load(),
		ContainmentNS:             recorder.scopeAlias.containmentNS.Load(),
		ReplacementNS:             recorder.scopeAlias.replacementNS.Load(),
		DurationNS:                recorder.scopeAlias.durationNS.Load(),
	}
}

type scopeAliasProbe struct {
	roots                  uint64
	recursiveVisits        uint64
	containmentCacheHits   uint64
	containmentCacheMisses uint64
	replacedRoots          uint64
	containmentDuration    time.Duration
	replacementDuration    time.Duration
}

func (recorder *PerfRecorder) recordScopeAliasProbe(probe scopeAliasProbe, duration time.Duration) {
	recorder.scopeAlias.calls.Add(1)
	recorder.scopeAlias.roots.Add(probe.roots)
	recorder.scopeAlias.recursiveVisits.Add(probe.recursiveVisits)
	recorder.scopeAlias.containmentCacheHits.Add(probe.containmentCacheHits)
	recorder.scopeAlias.containmentCacheMisses.Add(probe.containmentCacheMisses)
	recorder.scopeAlias.replacedRoots.Add(probe.replacedRoots)
	recorder.scopeAlias.containmentNS.Add(probe.containmentDuration.Nanoseconds())
	recorder.scopeAlias.replacementNS.Add(probe.replacementDuration.Nanoseconds())
	recorder.scopeAlias.durationNS.Add(duration.Nanoseconds())
}

func (recorder *PerfRecorder) recordScopeAliasContainmentCacheClear(entries int) {
	recorder.scopeAlias.containmentCacheClears.Add(1)
	if entries > 0 {
		recorder.scopeAlias.containmentEntriesEvicted.Add(uint64(entries))
	}
}

func (recorder *PerfRecorder) recordScopeAliasMutationEpoch(epoch uint64) {
	recorder.scopeAlias.mutationEpochAdvances.Add(1)
	for {
		current := recorder.scopeAlias.maxMutationEpoch.Load()
		if epoch <= current || recorder.scopeAlias.maxMutationEpoch.CompareAndSwap(current, epoch) {
			return
		}
	}
}

func (recorder *PerfRecorder) snapshotStaticAlias() StaticAliasPerfCounters {
	return StaticAliasPerfCounters{
		Calls:           recorder.staticAlias.calls.Load(),
		RefMisses:       recorder.staticAlias.refMisses.Load(),
		LocationVisits:  recorder.staticAlias.locationVisits.Load(),
		ChildHintHits:   recorder.staticAlias.childHintHits.Load(),
		DirectChildHits: recorder.staticAlias.directChildHits.Load(),
		Changed:         recorder.staticAlias.changed.Load(),
		NoChange:        recorder.staticAlias.noChange.Load(),
		DurationNS:      recorder.staticAlias.durationNS.Load(),
		CollectCalls:    recorder.staticAlias.collectCalls.Load(),
		CollectNS:       recorder.staticAlias.collectNS.Load(),
	}
}

func (recorder *PerfRecorder) snapshotStaticAliasTopFields(limit int) []StaticAliasFieldPerf {
	if limit <= 0 {
		return nil
	}
	recorder.staticAlias.mu.Lock()
	defer recorder.staticAlias.mu.Unlock()
	if len(recorder.staticAlias.fields) == 0 {
		return nil
	}
	out := make([]StaticAliasFieldPerf, 0, len(recorder.staticAlias.fields))
	for _, field := range recorder.staticAlias.fields {
		out = append(out, *field)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DurationNS == out[j].DurationNS {
			if out[i].Visits == out[j].Visits {
				return out[i].Field < out[j].Field
			}
			return out[i].Visits > out[j].Visits
		}
		return out[i].DurationNS > out[j].DurationNS
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (recorder *PerfRecorder) recordStaticAliasChildHintHit() {
	recorder.staticAlias.childHintHits.Add(1)
}

func (recorder *PerfRecorder) recordStaticAliasDirectChildHit() {
	recorder.staticAlias.directChildHits.Add(1)
}

func (recorder *PerfRecorder) recordStaticAliasPerf(duration time.Duration, refMiss bool, locationVisits uint64, changed bool) {
	recorder.staticAlias.calls.Add(1)
	if refMiss {
		recorder.staticAlias.refMisses.Add(1)
	}
	recorder.staticAlias.locationVisits.Add(locationVisits)
	if changed {
		recorder.staticAlias.changed.Add(1)
	} else {
		recorder.staticAlias.noChange.Add(1)
	}
	recorder.staticAlias.durationNS.Add(duration.Nanoseconds())
}

func (recorder *PerfRecorder) recordStaticAliasCollectPerf(duration time.Duration) {
	recorder.staticAlias.collectCalls.Add(1)
	recorder.staticAlias.collectNS.Add(duration.Nanoseconds())
}

func (recorder *PerfRecorder) recordStaticAliasFieldPerf(field, kind string, maxChildren int, duration time.Duration, changed bool) {
	if field == "" {
		return
	}
	recorder.staticAlias.mu.Lock()
	defer recorder.staticAlias.mu.Unlock()
	if recorder.staticAlias.fields == nil {
		recorder.staticAlias.fields = make(map[string]*StaticAliasFieldPerf)
	}
	entry := recorder.staticAlias.fields[field]
	if entry == nil {
		entry = &StaticAliasFieldPerf{Field: field}
		recorder.staticAlias.fields[field] = entry
	}
	entry.Visits++
	if changed {
		entry.Changed++
	} else {
		entry.NoChange++
	}
	entry.DurationNS += duration.Nanoseconds()
	if kind != "" {
		entry.Kind = kind
	}
	if maxChildren > entry.MaxChildren {
		entry.MaxChildren = maxChildren
	}
}

func recordStaticAliasPerf(duration time.Duration, refMiss bool, locationVisits uint64, changed bool) {
	if recorder := compatibilityPerfRecorder.Load(); recorder != nil {
		recorder.recordStaticAliasPerf(duration, refMiss, locationVisits, changed)
	}
}

func recordStaticAliasCollectPerf(duration time.Duration) {
	if recorder := compatibilityPerfRecorder.Load(); recorder != nil {
		recorder.recordStaticAliasCollectPerf(duration)
	}
}

func recordStaticAliasFieldPerf(field, kind string, maxChildren int, duration time.Duration, changed bool) {
	if recorder := compatibilityPerfRecorder.Load(); recorder != nil {
		recorder.recordStaticAliasFieldPerf(field, kind, maxChildren, duration, changed)
	}
}

func staticAliasPerfValueShape(value Value) (string, int) {
	switch value.Kind {
	case ValueObject:
		return string(value.Kind), len(value.Fields)
	case ValueMap:
		return string(value.Kind), len(value.Map) + len(value.MapKeys)
	case ValueList:
		return string(value.Kind), len(value.List)
	case ValueSet:
		return string(value.Kind), len(value.Set)
	default:
		return string(value.Kind), 0
	}
}

func (recorder *PerfRecorder) snapshotDML() DMLPerfCounters {
	return DMLPerfCounters{
		RollbackPoints:         recorder.dml.rollbackPoints.Load(),
		SnapshotRollbackPoints: recorder.dml.snapshotRollbackPoints.Load(),
		JournalRollbackPoints:  recorder.dml.journalRollbackPoints.Load(),
		TemporaryJournalPoints: recorder.dml.temporaryJournalPoints.Load(),
	}
}

func (vm *VM) recordSnapshotDMLRollbackPoint() {
	if vm == nil || vm.perfRecorder == nil {
		return
	}
	vm.perfRecorder.dml.rollbackPoints.Add(1)
	vm.perfRecorder.dml.snapshotRollbackPoints.Add(1)
}

func (vm *VM) recordJournalDMLRollbackPoint() {
	if vm == nil || vm.perfRecorder == nil {
		return
	}
	vm.perfRecorder.dml.rollbackPoints.Add(1)
	vm.perfRecorder.dml.journalRollbackPoints.Add(1)
}

func (vm *VM) recordTemporaryDMLJournalPoint() {
	if vm == nil || vm.perfRecorder == nil {
		return
	}
	vm.perfRecorder.dml.rollbackPoints.Add(1)
	vm.perfRecorder.dml.journalRollbackPoints.Add(1)
	vm.perfRecorder.dml.temporaryJournalPoints.Add(1)
}

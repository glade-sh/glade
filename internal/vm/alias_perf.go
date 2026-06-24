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
	StaticAliasTopFields []StaticAliasFieldPerf  `json:"staticAliasTopFields,omitempty"`
	DML                  DMLPerfCounters         `json:"dml,omitempty"`
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

var perfCountersEnabled atomic.Bool

var staticAliasPerf struct {
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

var dmlPerf struct {
	rollbackPoints         atomic.Uint64
	snapshotRollbackPoints atomic.Uint64
	journalRollbackPoints  atomic.Uint64
	temporaryJournalPoints atomic.Uint64
}

func SetPerfCountersEnabled(enabled bool) {
	perfCountersEnabled.Store(enabled)
}

func perfEnabled() bool {
	return perfCountersEnabled.Load()
}

func ResetPerfCounters() {
	perfCountersEnabled.Store(false)
	resetStaticAliasPerf()
	resetDMLPerf()
}

func SnapshotPerfCounters() PerfCounters {
	if !perfEnabled() {
		return PerfCounters{}
	}
	return PerfCounters{
		Enabled:              true,
		StaticAlias:          snapshotStaticAliasPerf(),
		StaticAliasTopFields: snapshotStaticAliasTopFields(20),
		DML:                  snapshotDMLPerf(),
	}
}

func resetStaticAliasPerf() {
	staticAliasPerf.calls.Store(0)
	staticAliasPerf.refMisses.Store(0)
	staticAliasPerf.locationVisits.Store(0)
	staticAliasPerf.childHintHits.Store(0)
	staticAliasPerf.directChildHits.Store(0)
	staticAliasPerf.changed.Store(0)
	staticAliasPerf.noChange.Store(0)
	staticAliasPerf.durationNS.Store(0)
	staticAliasPerf.collectCalls.Store(0)
	staticAliasPerf.collectNS.Store(0)
	staticAliasPerf.mu.Lock()
	staticAliasPerf.fields = nil
	staticAliasPerf.mu.Unlock()
}

func snapshotStaticAliasPerf() StaticAliasPerfCounters {
	return StaticAliasPerfCounters{
		Calls:           staticAliasPerf.calls.Load(),
		RefMisses:       staticAliasPerf.refMisses.Load(),
		LocationVisits:  staticAliasPerf.locationVisits.Load(),
		ChildHintHits:   staticAliasPerf.childHintHits.Load(),
		DirectChildHits: staticAliasPerf.directChildHits.Load(),
		Changed:         staticAliasPerf.changed.Load(),
		NoChange:        staticAliasPerf.noChange.Load(),
		DurationNS:      staticAliasPerf.durationNS.Load(),
		CollectCalls:    staticAliasPerf.collectCalls.Load(),
		CollectNS:       staticAliasPerf.collectNS.Load(),
	}
}

func recordStaticAliasChildHintHit() {
	if !perfEnabled() {
		return
	}
	staticAliasPerf.childHintHits.Add(1)
}

func recordStaticAliasDirectChildHit() {
	if !perfEnabled() {
		return
	}
	staticAliasPerf.directChildHits.Add(1)
}

func snapshotStaticAliasTopFields(limit int) []StaticAliasFieldPerf {
	if limit <= 0 {
		return nil
	}
	staticAliasPerf.mu.Lock()
	defer staticAliasPerf.mu.Unlock()
	if len(staticAliasPerf.fields) == 0 {
		return nil
	}
	out := make([]StaticAliasFieldPerf, 0, len(staticAliasPerf.fields))
	for _, field := range staticAliasPerf.fields {
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

func recordStaticAliasPerf(duration time.Duration, refMiss bool, locationVisits uint64, changed bool) {
	if !perfEnabled() {
		return
	}
	staticAliasPerf.calls.Add(1)
	if refMiss {
		staticAliasPerf.refMisses.Add(1)
	}
	staticAliasPerf.locationVisits.Add(locationVisits)
	if changed {
		staticAliasPerf.changed.Add(1)
	} else {
		staticAliasPerf.noChange.Add(1)
	}
	staticAliasPerf.durationNS.Add(duration.Nanoseconds())
}

func recordStaticAliasCollectPerf(duration time.Duration) {
	if !perfEnabled() {
		return
	}
	staticAliasPerf.collectCalls.Add(1)
	staticAliasPerf.collectNS.Add(duration.Nanoseconds())
}

func recordStaticAliasFieldPerf(field, kind string, maxChildren int, duration time.Duration, changed bool) {
	if !perfEnabled() || field == "" {
		return
	}
	staticAliasPerf.mu.Lock()
	defer staticAliasPerf.mu.Unlock()
	if staticAliasPerf.fields == nil {
		staticAliasPerf.fields = make(map[string]*StaticAliasFieldPerf)
	}
	entry := staticAliasPerf.fields[field]
	if entry == nil {
		entry = &StaticAliasFieldPerf{Field: field}
		staticAliasPerf.fields[field] = entry
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

func resetDMLPerf() {
	dmlPerf.rollbackPoints.Store(0)
	dmlPerf.snapshotRollbackPoints.Store(0)
	dmlPerf.journalRollbackPoints.Store(0)
	dmlPerf.temporaryJournalPoints.Store(0)
}

func snapshotDMLPerf() DMLPerfCounters {
	return DMLPerfCounters{
		RollbackPoints:         dmlPerf.rollbackPoints.Load(),
		SnapshotRollbackPoints: dmlPerf.snapshotRollbackPoints.Load(),
		JournalRollbackPoints:  dmlPerf.journalRollbackPoints.Load(),
		TemporaryJournalPoints: dmlPerf.temporaryJournalPoints.Load(),
	}
}

func (vm *VM) recordSnapshotDMLRollbackPoint() {
	if !perfEnabled() {
		return
	}
	dmlPerf.rollbackPoints.Add(1)
	dmlPerf.snapshotRollbackPoints.Add(1)
}

func (vm *VM) recordJournalDMLRollbackPoint() {
	if !perfEnabled() {
		return
	}
	dmlPerf.rollbackPoints.Add(1)
	dmlPerf.journalRollbackPoints.Add(1)
}

func (vm *VM) recordTemporaryDMLJournalPoint() {
	if !perfEnabled() {
		return
	}
	dmlPerf.rollbackPoints.Add(1)
	dmlPerf.journalRollbackPoints.Add(1)
	dmlPerf.temporaryJournalPoints.Add(1)
}

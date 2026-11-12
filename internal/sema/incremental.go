package sema

import (
	"crypto/sha256"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// IncrementalState lets a long-lived caller (the test daemon) analyze a
// sequence of generations of one project and redo only what an edit can change.
//
// The result of every Analyze call equals AnalyzeWithOptions on the same
// input. The state reuses one thing: the per-type method-body diagnostics of
// the previous generation. A type's body diagnostics are a function of its own
// source and companion metadata, the member model, the index, and the
// analysis options. After the enrichment step the index is hashed without
// source positions. When that hash matches the previous generation's, no
// signature, member, annotation, schema, dependency or metadata fact changed
// anywhere; the only difference is which files changed content, and those
// types, plus the types that name them or extend them, are checked again.
// Every other type keeps its cached diagnostics. Any other difference runs the
// whole analysis, as does any run that shows the cache could be stale.
//
// Reuse is sound only while a type's diagnostics do not depend on what other
// types hydrated into the member view. Each analysis checks bodies on a
// probed view; if any lookup answers "no such field", or answers "yes" only
// through a hydrated entry, the run is serial history dependent: it is
// computed exactly as AnalyzeWithOptions computes it and is not cached.
type IncrementalState struct {
	mu   sync.Mutex
	prev *incrementalSnapshot
	last IncrementalStats
}

// LastStats reports what the most recent Analyze call did.
func (s *IncrementalState) LastStats() IncrementalStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

// NewIncrementalState returns an empty state; its first Analyze is a full run.
func NewIncrementalState() *IncrementalState { return &IncrementalState{} }

// IncrementalStats reports what one Analyze call did.
type IncrementalStats struct {
	// Incremental is true when cached body diagnostics were reused.
	Incremental bool
	// Reason names why a run was not incremental; empty when it was.
	Reason string
	// Types counts the types with body work in this generation.
	Types int
	// Rechecked counts the types whose bodies were checked in this call.
	Rechecked int
	// Reused counts the types whose cached diagnostics were returned.
	Reused int
}

type incrementalSnapshot struct {
	optionsFP   string
	indexDigest [sha256.Size]byte
	typeDigests [][sha256.Size]byte
	typeDiags   [][]diagnostic.Diagnostic
}

// incrementalRun is the per-call working state threaded through one analysis.
type incrementalRun struct {
	prev      *incrementalSnapshot
	optionsFP string
	artifacts *typesys.BuildArtifacts

	indexDigest [sha256.Size]byte
	typeDigests [][sha256.Size]byte
	digestsOK   bool
	reuseOK     bool
	reason      string
	// changed marks types whose source or companion metadata differ from prev.
	changed []bool
	// recheck marks the types whose bodies run in this call; nil checks all.
	recheck []bool

	snapshot *incrementalSnapshot
	stats    IncrementalStats
	fallback bool
	finished bool

	laterProbe atomic.Bool
	laterUsed  atomic.Bool
	armed      bool
}

// Analyze analyzes index with opts, reusing the previous call's per-type body
// diagnostics where they provably still hold. Calls are serialized.
func (s *IncrementalState) Analyze(index typesys.Index, opts AnalyzeOptions) (Result, IncrementalStats) {
	if opts.BuildArtifacts == nil || opts.CapturedSource != nil || !opts.Diagnostics {
		return AnalyzeWithOptions(index, opts), IncrementalStats{Reason: "no captured build artifacts"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, run := s.analyzeLocked(index, opts, s.prev)
	if run.fallback {
		result, run = s.analyzeLocked(index, opts, nil)
		run.stats.Reason = "stale cache risk"
	}
	s.prev = run.snapshot
	s.last = run.stats
	return result, run.stats
}

func (s *IncrementalState) analyzeLocked(index typesys.Index, opts AnalyzeOptions, prev *incrementalSnapshot) (Result, *incrementalRun) {
	run := &incrementalRun{
		prev:      prev,
		optionsFP: AnalyzeOptionsFingerprint(opts),
		artifacts: opts.BuildArtifacts,
	}
	var result Result
	if opts.PerfCounters == nil || !opts.PerfCounters.Enabled {
		result = NewAnalyzer().analyzeWithOptionsIncremental(index, opts, nil, run)
	} else {
		recorder := newPerfRecorder(opts.PerfCounters)
		platformStarted := recorder.beginPhase()
		analyzer := NewAnalyzer()
		recorder.endPhase(&recorder.counters.PlatformModel, platformStarted)
		result = analyzer.analyzeWithOptionsIncremental(index, opts, &recorder, run)
		recorder.finish()
	}
	if !run.finished {
		// A panic ended the analysis early. The result carries the recovered
		// diagnostic, exactly as a plain run does, and nothing is cached.
		run.snapshot = nil
		if prev != nil && run.reuseOK {
			run.fallback = true
		}
	}
	return result, run
}

// begin hashes the enriched index and the per-type source identities and
// decides whether the previous snapshot can serve this generation.
func (r *incrementalRun) begin(index typesys.Index, opts AnalyzeOptions) {
	// Project markers decide which source unit each file belongs to; the member
	// model reads them from disk, so they are part of the index's identity.
	r.indexDigest = positionBlindDigest(struct {
		Index   typesys.Index
		Markers map[string]bool
	}{index, anonymousProjectMarkers(index)})
	r.typeDigests = make([][sha256.Size]byte, len(index.Types))
	r.digestsOK = true
	for i, typ := range index.Types {
		if !typ.HasSourceSnapshot() {
			continue
		}
		source, ok := r.artifacts.SourceForType(typ)
		metadata, metaOK := r.artifacts.ApexMetadataForType(typ)
		if !ok || !metaOK {
			r.digestsOK = false
			r.reason = "missing captured source"
			return
		}
		hash := sha256.New()
		digest := source.Digest()
		hash.Write(digest[:])
		if metadata.Present {
			hash.Write([]byte{1})
		} else {
			hash.Write([]byte{0})
		}
		hash.Write(metadata.Digest[:])
		hash.Sum(r.typeDigests[i][:0])
	}
	switch {
	case r.prev == nil:
		r.reason = "first analysis"
	case r.prev.optionsFP != r.optionsFP:
		r.reason = "options changed"
	case r.prev.indexDigest != r.indexDigest || len(r.prev.typeDigests) != len(r.typeDigests):
		r.reason = "signature, member or metadata change"
	default:
		r.reuseOK = true
	}
	if !r.reuseOK {
		return
	}
	r.changed = make([]bool, len(index.Types))
	for i := range r.typeDigests {
		r.changed[i] = r.typeDigests[i] != r.prev.typeDigests[i]
	}
}

// incrementalRecheckSet marks every type that must run again: the changed
// types, the types whose source names a changed top-level type, and every
// type that extends or implements one of those, transitively.
func (r *incrementalRun) expandDependents(a *Analyzer, index typesys.Index) {
	if !r.reuseOK {
		return
	}
	recheck := append([]bool(nil), r.changed...)
	names := map[string]bool{}
	anyChanged := false
	for i, typ := range index.Types {
		if r.changed[i] {
			anyChanged = true
			names[strings.ToLower(topLevelTypeName(typ.Name))] = true
		}
	}
	if anyChanged {
		// Subtypes: closure over superclass and interface names.
		for grown := true; grown; {
			grown = false
			for i, typ := range index.Types {
				if recheck[i] {
					continue
				}
				if extendsAny(typ, names) {
					recheck[i] = true
					names[strings.ToLower(topLevelTypeName(typ.Name))] = true
					grown = true
				}
			}
		}
		// Mentions: a body or declaration that spells a changed name.
		needles := make([]string, 0, len(names))
		for name := range names {
			needles = append(needles, name)
		}
		for i, typ := range index.Types {
			if recheck[i] || skipProjectDiagnosticType(typ) || !typ.HasSourceSnapshot() {
				continue
			}
			source, ok := a.sources.normalizedForType(typ)
			if !ok {
				recheck[i] = true
				continue
			}
			lowered := strings.ToLower(source)
			for _, needle := range needles {
				if strings.Contains(lowered, needle) {
					recheck[i] = true
					break
				}
			}
		}
	}
	r.recheck = recheck
}

func topLevelTypeName(name string) string {
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		return name[:dot]
	}
	return name
}

func extendsAny(typ typesys.TypeSymbol, names map[string]bool) bool {
	if names[strings.ToLower(topLevelTypeName(lastNameSegmentBeforeGeneric(typ.SuperClass)))] {
		return true
	}
	for _, iface := range typ.Interfaces {
		if names[strings.ToLower(topLevelTypeName(lastNameSegmentBeforeGeneric(iface)))] {
			return true
		}
	}
	return false
}

// lastNameSegmentBeforeGeneric drops type arguments; a qualified name keeps
// its first segment through topLevelTypeName, so Outer.Inner maps to Outer.
func lastNameSegmentBeforeGeneric(name string) string {
	if lt := strings.IndexByte(name, '<'); lt >= 0 {
		name = name[:lt]
	}
	return strings.TrimSpace(name)
}

// armLaterPasses watches the passes after the body pass. They run on a view
// that holds only what the rechecked types hydrated, so any answer that
// depends on the hydration history sends the call back to a full analysis.
func (r *incrementalRun) armLaterPasses(state *semaTypeMemberState) {
	if !r.reuseOK || state == nil {
		return
	}
	state.historyProbe = &r.laterProbe
	state.historyUsed = &r.laterUsed
	r.armed = true
}

// finish closes a successful analysis.
func (r *incrementalRun) finish() {
	if r.armed && (r.laterProbe.Load() || r.laterUsed.Load()) {
		r.fallback = true
		r.snapshot = nil
	}
	r.finished = true
}

// checkMethodBodiesIncremental is checkMethodBodiesWithView with per-type
// reuse. Its output is identical to the plain pass. It always checks on a
// probed view, so a run it caches never depended on hydration history.
func (a *Analyzer) checkMethodBodiesIncremental(index typesys.Index, model *semaTypeMemberView, recorder *perfRecorder, incr *incrementalRun) []diagnostic.Diagnostic {
	plain := func() []diagnostic.Diagnostic {
		incr.snapshot = nil
		return a.checkMethodBodiesWithViewWorkers(index, model, recorder, 1)
	}
	if model == nil || model.state == nil || !incr.digestsOK {
		return plain()
	}
	started := recorder.beginPhase()
	incr.expandDependents(a, index)
	var probe, used atomic.Bool
	probed := &semaTypeMemberState{base: model.state.base, platform: model.state.platform, historyProbe: &probe, historyUsed: &used}
	scratch := model.cloneOnto(probed)
	var include func(int) bool
	if incr.recheck != nil {
		include = func(typeIndex int) bool { return incr.recheck[typeIndex] }
	}
	workers := semaMethodBodyWorkerCount()
	fieldDiagnostics, results, views, recovered := a.checkMethodBodiesPartial(index, scratch, workers, include, &probe)
	if probe.Load() || used.Load() {
		if incr.reuseOK {
			incr.fallback = true
			return nil
		}
		return plain()
	}
	if recovered != nil {
		if incr.reuseOK {
			incr.fallback = true
			return nil
		}
		panic(recovered)
	}
	for _, view := range views {
		model.mergeHydrated(view)
	}
	diagnostics := fieldDiagnostics
	snapshot := &incrementalSnapshot{
		optionsFP:   incr.optionsFP,
		indexDigest: incr.indexDigest,
		typeDigests: incr.typeDigests,
		typeDiags:   make([][]diagnostic.Diagnostic, len(index.Types)),
	}
	stats := IncrementalStats{Incremental: incr.reuseOK, Reason: incr.reason}
	byType := make([][]diagnostic.Diagnostic, len(index.Types))
	for typeIndex, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || !semaTypeHasMethodBodyWork(typ) {
			continue
		}
		if incr.recheck != nil && !incr.recheck[typeIndex] {
			cached := incr.prev.typeDiags[typeIndex]
			byType[typeIndex] = cloneDiagnostics(cached)
			snapshot.typeDiags[typeIndex] = cached
			stats.Reused++
		} else {
			byType[typeIndex] = results[typeIndex].diagnostics
			snapshot.typeDiags[typeIndex] = cloneDiagnostics(results[typeIndex].diagnostics)
			stats.Rechecked++
		}
		stats.Types++
	}
	for _, typeIndex := range semaInnerFirstTypeOrder(index) {
		diagnostics = append(diagnostics, byType[typeIndex]...)
	}
	incr.snapshot = snapshot
	incr.stats = stats
	if recorder != nil {
		recorder.endPhase(&recorder.counters.MethodBodies, started)
	}
	return diagnostics
}

func cloneDiagnostics(items []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	if items == nil {
		return nil
	}
	out := make([]diagnostic.Diagnostic, len(items))
	copy(out, items)
	for i := range out {
		if out[i].Range != nil {
			r := *out[i].Range
			out[i].Range = &r
		}
		if out[i].NativeLine != nil {
			n := *out[i].NativeLine
			out[i].NativeLine = &n
		}
	}
	return out
}

// checkMethodBodiesPartial runs the field-initializer pass and the body task
// of every type include admits (nil admits all) on the probed scratch view,
// with up to workers goroutines. It mirrors checkMethodBodiesRun's parallel
// branch, so each type's view and result match what that branch produces. The
// returned views are every worker's view, primary first.
func (a *Analyzer) checkMethodBodiesPartial(index typesys.Index, scratch *semaTypeMemberView, workers int, include func(int) bool, abort *atomic.Bool) (fieldDiagnostics []diagnostic.Diagnostic, results []semaMethodBodyResult, views []*semaTypeMemberView, recovered any) {
	defer func() {
		if r := recover(); r != nil {
			recovered = r
		}
	}()
	constructability := buildConstructability(index)
	duplicateTypes := semaDuplicateTypeKeys(index)
	sources := a.sources
	if sources == nil {
		sources = newSemaSources(nil, nil)
	}
	fieldDiagnostics = a.checkFieldInitializerTypes(index, scratch, sources)
	workItems := buildSemaMethodBodyWorkItemsFor(index, sources, include)
	tasks := buildSemaMethodBodyTasks(index, workItems)
	if workers > len(tasks) {
		workers = len(tasks)
	}
	if workers < 1 {
		workers = 1
	}
	results = make([]semaMethodBodyResult, len(index.Types))
	views = make([]*semaTypeMemberView, workers)
	views[0] = scratch
	for workerID := 1; workerID < workers; workerID++ {
		views[workerID] = scratch.cloneOnto(scratch.state)
	}
	var group sync.WaitGroup
	for workerID := 0; workerID < workers; workerID++ {
		workerID := workerID
		group.Add(1)
		go func() {
			defer group.Done()
			for taskIndex := workerID; taskIndex < len(tasks); taskIndex += workers {
				if abort != nil && abort.Load() {
					return
				}
				task := tasks[taskIndex]
				results[task.typeIndex] = a.checkSemaMethodBodyTask(index, task, workItems, views[workerID], duplicateTypes, constructability)
			}
		}()
	}
	group.Wait()
	for typeIndex := range results {
		if results[typeIndex].recovered != nil {
			recovered = results[typeIndex].recovered
			return
		}
	}
	return
}

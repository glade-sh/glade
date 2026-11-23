package sema

import (
	"crypto/sha256"
	"strings"
	"sync"

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
// A serial body pass lets a type see what earlier types hydrated into the
// member view, and semaAnyKnownField reads that history. Each type's body
// task records which answers depended on it and which fields the type added
// (incremental_hydration.go). A reused type keeps its cached diagnostics only
// while every such answer is still what the serial pass would give; a type
// whose answer may have changed is checked again. A run whose answers cannot
// be settled this way is computed exactly as AnalyzeWithOptions computes it
// and is not cached.
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
	// typeHydration holds each type's body-pass hydration record, and ever
	// every hydrated field any generation so far saw made visible.
	typeHydration []*semaTypeHydration
	ever          map[string]struct{}
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

	// hydration is this generation's per-type records, and visible the fields
	// they make visible; the passes after the body pass answer from both.
	hydration []*semaTypeHydration
	visible   map[string]int
	ever      map[string]struct{}
	later     *semaLaterHistory
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
		if run.fallback {
			// Even a run that checks every type could not settle its answers.
			result, run = AnalyzeWithOptions(index, opts), &incrementalRun{}
		}
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

// expandDependents sets recheck to every type that must run again: the changed
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
				if containsIdentifierMention(source, lowered, needle) {
					recheck[i] = true
					break
				}
			}
		}
	}
	r.recheck = recheck
}

// containsIdentifierMention keeps uncertain non-ASCII and escaped source
// conservative while excluding occurrences inside longer ASCII identifiers.
func containsIdentifierMention(source, lowered, name string) bool {
	first := strings.Index(lowered, name)
	if first < 0 {
		return false
	}
	if name == "" || !isASCIIIdentifier(name) {
		return true
	}
	for i := 0; i < len(source); i++ {
		if source[i] >= 0x80 || source[i] == '\\' {
			return true
		}
	}
	for i := first; i >= 0; {
		end := i + len(name)
		// A digit before a name could start a separate numeric token in
		// malformed source, so only letters and underscore rule it out.
		left := i > 0 && isASCIIIdentifierByte(lowered[i-1]) && (lowered[i-1] < '0' || lowered[i-1] > '9')
		right := end < len(lowered) && isASCIIIdentifierByte(lowered[end])
		if !left && !right {
			return true
		}
		next := strings.Index(lowered[i+1:], name)
		if next < 0 {
			return false
		}
		i += next + 1
	}
	return false
}

func isASCIIIdentifier(name string) bool {
	for i := 0; i < len(name); i++ {
		if !isASCIIIdentifierByte(name[i]) {
			return false
		}
	}
	return true
}

func isASCIIIdentifierByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
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

// armLaterPasses hands the passes after the body pass the body history the
// run did not merge into their view. Rechecked types ran on private views, and
// a run that was checked again may have hydrated what the serial pass does
// not, so only the pre-body history is merged. A field some valid type made
// visible is "known" there, as it is on the serial history; finish accepts an
// "unknown" answer only for a field no type touched.
func (r *incrementalRun) armLaterPasses(state *semaTypeMemberState) {
	if state == nil || r.snapshot == nil {
		return
	}
	r.later = &semaLaterHistory{visible: r.visible}
	state.later = r.later
}

// finish closes a successful analysis.
func (r *incrementalRun) finish() {
	if r.later != nil {
		for name := range r.later.missingNames() {
			if _, possible := r.ever[name]; possible {
				r.fallback = true
				r.snapshot = nil
				break
			}
		}
	}
	r.finished = true
}

// semaIncrementalMaxRounds bounds the recheck rounds of one body pass. Each
// round checks the reused types whose recorded answers no longer hold.
const semaIncrementalMaxRounds = 4

// checkMethodBodiesIncremental is checkMethodBodiesWithView with per-type
// reuse. Its output is identical to the plain pass. Every type it checks runs
// on a traced view that starts from the pre-body history, and the records are
// validated in serial order before anything is reused or cached.
func (a *Analyzer) checkMethodBodiesIncremental(index typesys.Index, model *semaTypeMemberView, recorder *perfRecorder, incr *incrementalRun) []diagnostic.Diagnostic {
	plain := func() []diagnostic.Diagnostic {
		incr.snapshot = nil
		return a.checkMethodBodiesWithViewWorkers(index, model, recorder, 1)
	}
	giveUp := func() []diagnostic.Diagnostic {
		if incr.reuseOK {
			incr.fallback = true
			return nil
		}
		return plain()
	}
	if model == nil || model.state == nil || !incr.digestsOK {
		return plain()
	}
	started := recorder.beginPhase()
	incr.expandDependents(a, index)
	probed := &semaTypeMemberState{base: model.state.base, platform: model.state.platform}
	pre := model.cloneOnto(probed)
	fieldDiagnostics, recovered := a.checkFieldInitializersRecovering(index, pre)
	if recovered != nil {
		if incr.reuseOK {
			incr.fallback = true
			return nil
		}
		panic(recovered)
	}
	preFields := semaHydratedFieldSet(pre)
	typeCount := len(index.Types)
	results := make([]semaMethodBodyResult, typeCount)
	records := make([]*semaTypeHydration, typeCount)
	rechecked := make([]bool, typeCount)
	include := func(typeIndex int) bool { return incr.recheck == nil || incr.recheck[typeIndex] }
	var oracle map[string]int
	var ever map[string]struct{}
	if incr.prev != nil {
		ever = incr.prev.ever
	}
	if incr.recheck != nil {
		// The reused types' records give the first round its oracle.
		for typeIndex := range records {
			if !incr.recheck[typeIndex] {
				records[typeIndex] = incr.prev.typeHydration[typeIndex]
			}
		}
		oracle = semaValidateHydration(records, rechecked, preFields, ever).firstSelf
	}
	workers := semaMethodBodyWorkerCount()
	var check semaHydrationCheck
	for round := 1; ; round++ {
		roundResults, roundRecords, unsafe, recovered := a.checkMethodBodiesTraced(index, pre, preFields, oracle, workers, round, include)
		if recovered != nil {
			if incr.reuseOK {
				incr.fallback = true
				return nil
			}
			panic(recovered)
		}
		if unsafe {
			return giveUp()
		}
		for typeIndex := range index.Types {
			if include(typeIndex) {
				rechecked[typeIndex] = true
				results[typeIndex] = roundResults[typeIndex]
				records[typeIndex] = roundRecords[typeIndex]
			}
		}
		check = semaValidateHydration(records, rechecked, preFields, ever)
		if check.conflict || (check.reruns != 0 && round == semaIncrementalMaxRounds) {
			return giveUp()
		}
		if check.reruns == 0 {
			break
		}
		again := check.rerun
		include = func(typeIndex int) bool { return again[typeIndex] }
		oracle = check.firstSelf
	}
	model.mergeHydrated(pre)
	diagnostics := fieldDiagnostics
	snapshot := &incrementalSnapshot{
		optionsFP:     incr.optionsFP,
		indexDigest:   incr.indexDigest,
		typeDigests:   incr.typeDigests,
		typeDiags:     make([][]diagnostic.Diagnostic, typeCount),
		typeHydration: records,
		ever:          semaEverVisible(ever, preFields, records),
	}
	stats := IncrementalStats{Incremental: incr.reuseOK, Reason: incr.reason}
	byType := make([][]diagnostic.Diagnostic, typeCount)
	for typeIndex, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || !semaTypeHasMethodBodyWork(typ) {
			continue
		}
		if !rechecked[typeIndex] {
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
	incr.hydration = records
	incr.visible = check.firstSelf
	incr.ever = snapshot.ever
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

// checkFieldInitializersRecovering runs the field-initializer pass on view
// and hands back a panic instead of raising it.
func (a *Analyzer) checkFieldInitializersRecovering(index typesys.Index, view *semaTypeMemberView) (diagnostics []diagnostic.Diagnostic, recovered any) {
	defer func() {
		if r := recover(); r != nil {
			recovered = r
		}
	}()
	sources := a.sources
	if sources == nil {
		sources = newSemaSources(nil, nil)
	}
	return a.checkFieldInitializerTypes(index, view, sources), nil
}

// checkMethodBodiesTraced runs the body task of every type include admits on
// up to workers goroutines. Each worker starts from its own copy of pre and
// carries a trace with the oracle; each task starts with empty view memos. It
// mirrors checkMethodBodiesRun's parallel branch: worker w takes tasks w,
// w+n, ..., in serial order.
func (a *Analyzer) checkMethodBodiesTraced(index typesys.Index, pre *semaTypeMemberView, preFields map[string]struct{}, oracle map[string]int, workers, round int, include func(int) bool) (results []semaMethodBodyResult, records []*semaTypeHydration, unsafe bool, recovered any) {
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
	workItems := buildSemaMethodBodyWorkItemsFor(index, sources, include)
	var tasks []semaMethodBodyTask
	for _, task := range buildSemaMethodBodyTasks(index, workItems) {
		if include(task.typeIndex) {
			tasks = append(tasks, task)
		}
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}
	if workers < 1 {
		workers = 1
	}
	results = make([]semaMethodBodyResult, len(index.Types))
	records = make([]*semaTypeHydration, len(index.Types))
	views := make([]*semaTypeMemberView, workers)
	traces := make([]*semaHydrationTrace, workers)
	for workerID := range views {
		views[workerID] = pre.cloneOnto(pre.state)
		traces[workerID] = newSemaHydrationTrace(preFields, oracle, workerID, round)
		views[workerID].trace = traces[workerID]
	}
	var group sync.WaitGroup
	for workerID := 0; workerID < workers; workerID++ {
		workerID := workerID
		group.Add(1)
		go func() {
			defer group.Done()
			view, trace := views[workerID], traces[workerID]
			for taskIndex := workerID; taskIndex < len(tasks); taskIndex += workers {
				task := tasks[taskIndex]
				record := trace.begin(task.typeIndex)
				view.resetBodyMemos()
				results[task.typeIndex] = a.checkSemaMethodBodyTask(index, task, workItems, view, duplicateTypes, constructability)
				trace.cur = nil
				records[task.typeIndex] = record
			}
		}()
	}
	group.Wait()
	for _, trace := range traces {
		unsafe = unsafe || trace.unsafe
	}
	for typeIndex := range results {
		if results[typeIndex].recovered != nil {
			recovered = results[typeIndex].recovered
			return
		}
	}
	return
}

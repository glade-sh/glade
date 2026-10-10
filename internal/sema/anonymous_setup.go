package sema

import (
	"crypto/sha256"
	"encoding/json"
	"maps"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// anonymousSetup holds the analysis tables AnalyzeAnonymous derives from the
// index alone. Nothing in it depends on the anonymous source text, so one
// setup serves every body analyzed against the same index. Every field is
// read-only after buildAnonymousSetup returns; per-call state is created by
// analyzer and queryChecker.
type anonymousSetup struct {
	index                 typesys.Index
	known                 map[string]TypeReference
	namespace             string
	deps                  map[string]bool
	queryDeclaredObjects  []schema.Object
	visualforcePageNames  map[string]bool
	visualforcePagesKnown bool
	typeMembers           *semaTypeMemberState
	queryChecker          querySemanticsChecker
	// sourceReads records every project source file the setup read from disk,
	// by path, and projectMarkers every sfdx-project.json existence check the
	// type-member model can make. A cached setup is reused only while both
	// still hold, so live-file behavior matches a fresh setup.
	sourceReads    map[string]anonymousSourceRead
	projectMarkers map[string]bool
}

type anonymousSourceRead struct {
	raw string
	ok  bool
}

// buildAnonymousSetup also reports whether the setup may be cached: false when
// a source file changed while the setup was reading it.
func buildAnonymousSetup(index typesys.Index) (*anonymousSetup, bool) {
	// Markers are recorded before the build. A marker that changes during the
	// build then differs from the record on the next hit and forces a rebuild.
	markers := anonymousProjectMarkers(index)
	analyzer := NewAnalyzer()
	analyzer.prepareAnalysisContext(index, AnalyzeOptions{}, nil)
	index = prepareAnalysisIndexWithSources(index, analyzer.sources)
	analyzer.prepareAnalysisModel(index)
	state := buildSemaTypeMemberState(index, nil, analyzer.sources)
	queryChecker := newQuerySemanticsChecker(index, analyzer.queryDeclaredObjects...)
	if hooks := anonymousSetupTestHooks.Load(); hooks != nil && hooks.afterSourceReads != nil {
		hooks.afterSourceReads(index)
	}
	for marker, exists := range anonymousProjectMarkers(index) {
		if _, recorded := markers[marker]; !recorded {
			markers[marker] = exists
		}
	}
	cacheable := true
	reads := make(map[string]anonymousSourceRead, len(analyzer.sources.fallback))
	for key, result := range analyzer.sources.fallback {
		path, _, _ := strings.Cut(key, "\x00")
		read := anonymousSourceRead{raw: result.text.raw, ok: result.ok}
		if !read.matchesFile(path) {
			// The file changed while the setup was reading it.
			cacheable = false
		}
		reads[path] = read
	}
	return &anonymousSetup{
		index:                 index,
		known:                 analyzer.known,
		namespace:             analyzer.namespace,
		deps:                  analyzer.deps,
		queryDeclaredObjects:  analyzer.queryDeclaredObjects,
		visualforcePageNames:  analyzer.visualforcePageNames,
		visualforcePagesKnown: analyzer.visualforcePagesKnown,
		typeMembers:           state,
		queryChecker:          queryChecker,
		sourceReads:           reads,
		projectMarkers:        markers,
	}, cacheable
}

// anonymousProjectMarkers records whether sfdx-project.json exists at every
// path semaSourceUnitMarkerWalk can probe for the index's type files. It takes
// the walk to its end instead of stopping at the first marker, so it covers
// the checks a build makes under any marker layout.
func anonymousProjectMarkers(index typesys.Index) map[string]bool {
	markers := make(map[string]bool)
	for _, typ := range index.Types {
		_, dir, ok := semaSourceUnitFile(typ.File)
		if !ok {
			continue
		}
		semaSourceUnitMarkerWalk(dir, typ.SourceRoot, func(marker string) bool {
			if _, seen := markers[marker]; !seen {
				_, err := os.Stat(marker)
				markers[marker] = err == nil
			}
			return false
		})
	}
	return markers
}

// analyzer returns a fresh analyzer over the shared known-type table. Only
// addKnown and prepareAnalysisModel write known, and neither runs on the
// anonymous body path. deps, queryDeclaredObjects, the canonical-name cache
// and the source resolver are per call.
func (s *anonymousSetup) analyzer() *Analyzer {
	return &Analyzer{
		known:                 s.known,
		canonicalNames:        newSemaCanonicalNames(semaAnalysisCanonicalNameLimit),
		namespace:             s.namespace,
		deps:                  maps.Clone(s.deps),
		queryDeclaredObjects:  append([]schema.Object(nil), s.queryDeclaredObjects...),
		visualforcePageNames:  s.visualforcePageNames,
		visualforcePagesKnown: s.visualforcePagesKnown,
		sources:               newSemaSourcesWithCaptured(nil, nil, nil),
	}
}

// newQueryChecker copies the maps that querySemanticsChecker.object fills
// lazily while checking. knownTypes is only written by the constructor.
// Field providers memoize pure lookups and are safe to share.
func (s *anonymousSetup) newQueryChecker() querySemanticsChecker {
	checker := s.queryChecker
	checker.objects = maps.Clone(s.queryChecker.objects)
	checker.providers = maps.Clone(s.queryChecker.providers)
	checker.declaredFields = maps.Clone(s.queryChecker.declaredFields)
	return checker
}

// inputsUnchanged reports whether every file and marker the setup depended on
// is as it was. Every recorded source is reread and compared byte for byte on
// each hit: size and modification time cannot prove content unchanged, since
// copies, checkouts and coarse timestamps keep them across edits.
func (s *anonymousSetup) inputsUnchanged() bool {
	for marker, existed := range s.projectMarkers {
		if _, err := os.Stat(marker); (err == nil) != existed {
			return false
		}
	}
	for path, read := range s.sourceReads {
		if !read.matchesFile(path) {
			return false
		}
	}
	return true
}

func (r anonymousSourceRead) matchesFile(path string) bool {
	data, err := os.ReadFile(path) // #nosec G304 -- path was read by the same setup from an indexed project source.
	if err != nil {
		return !r.ok
	}
	return r.ok && string(data) == r.raw
}

// anonymousSetupKey is the content identity of the inputs a setup reads: the
// same components semanticcache.IdentityForBuild binds (project root, index
// content, source digests, private project identity) plus the API version.
// sema cannot import semanticcache, and AnalyzeAnonymous has no build
// artifacts, so the digest is computed here from the index alone.
type anonymousSetupKey [sha256.Size]byte

// anonymousHiddenSymbolState carries the index fields JSON omits.
type anonymousHiddenSymbolState struct {
	ConstructorsAuthoritative bool
	EnumHashBase              *int64
}

type anonymousSourceDigest struct {
	File   string
	Digest [sha256.Size]byte
	OK     bool
}

func newAnonymousSetupKey(index typesys.Index, apiVersion string) (anonymousSetupKey, bool) {
	hidden := make([]anonymousHiddenSymbolState, len(index.Types))
	for i, typ := range index.Types {
		hidden[i] = anonymousHiddenSymbolState{ConstructorsAuthoritative: typ.ConstructorsAuthoritative, EnumHashBase: typ.EnumHashBase}
	}
	var inferredChildRelationships []bool
	for _, object := range index.Objects {
		for _, field := range object.Fields {
			inferredChildRelationships = append(inferredChildRelationships, field.ChildRelationshipNameInferred)
		}
	}
	digests := make([]anonymousSourceDigest, 0, len(index.Types)+len(index.Triggers))
	addDigest := func(file string) {
		digest, ok := index.SourceDigest(file)
		digests = append(digests, anonymousSourceDigest{File: file, Digest: digest, OK: ok})
	}
	for _, typ := range index.Types {
		addDigest(typ.File)
	}
	for _, trigger := range index.Triggers {
		addDigest(trigger.File)
	}
	projectIdentity, hasProjectIdentity := typesys.ProjectIdentityDigest(index)
	hash := sha256.New()
	err := json.NewEncoder(hash).Encode(struct {
		Root                       string
		APIVersion                 string
		Index                      typesys.Index
		Hidden                     []anonymousHiddenSymbolState
		InferredChildRelationships []bool
		SourceDigests              []anonymousSourceDigest
		ProjectIdentity            [sha256.Size]byte
		HasProjectIdentity         bool
	}{index.Project.Root, apiVersion, index, hidden, inferredChildRelationships, digests, projectIdentity, hasProjectIdentity})
	if err != nil {
		return anonymousSetupKey{}, false
	}
	var key anonymousSetupKey
	hash.Sum(key[:0])
	return key, true
}

// anonymousSetupCacheLimit bounds the setups a long-lived process retains.
// Conformance tests alternate a few indexes and API versions; a server sees
// one project index at a time.
const anonymousSetupCacheLimit = 8

var anonymousSetupCache struct {
	mu       sync.Mutex
	entries  map[anonymousSetupKey]*anonymousSetup
	building map[anonymousSetupKey]*anonymousSetupBuild
	keys     map[anonymousIndexIdentity]anonymousSetupKey
}

// anonymousSetupBuild lets concurrent misses on one key wait for a single
// build instead of building duplicates. setup is set only when the build is
// cacheable; waiters still revalidate it exactly as a cache hit does.
type anonymousSetupBuild struct {
	done  chan struct{}
	setup *anonymousSetup
}

// anonymousSetupTestHooks lets tests hold a build after it has read its
// sources and observe a call joining an in-flight build. It is nil outside
// tests.
var anonymousSetupTestHooks atomic.Pointer[anonymousSetupHooks]

type anonymousSetupHooks struct {
	afterSourceReads func(typesys.Index)
	waiting          func(typesys.Index)
}

func anonymousSetupFor(index typesys.Index, apiVersion string) *anonymousSetup {
	key, cacheable := anonymousSetupKeyFor(index, apiVersion)
	if !cacheable {
		setup, _ := buildAnonymousSetup(index)
		return setup
	}
	for {
		anonymousSetupCache.mu.Lock()
		if setup := anonymousSetupCache.entries[key]; setup != nil {
			anonymousSetupCache.mu.Unlock()
			if setup.inputsUnchanged() {
				return setup
			}
			anonymousSetupCache.mu.Lock()
			if anonymousSetupCache.entries[key] == setup {
				delete(anonymousSetupCache.entries, key)
			}
			anonymousSetupCache.mu.Unlock()
			continue
		}
		if build := anonymousSetupCache.building[key]; build != nil {
			anonymousSetupCache.mu.Unlock()
			if hooks := anonymousSetupTestHooks.Load(); hooks != nil && hooks.waiting != nil {
				hooks.waiting(index)
			}
			<-build.done
			// The leader's build may predate an edit this call must see. Use it
			// only if it was cacheable and still matches every input; otherwise
			// build again.
			if setup := build.setup; setup != nil && setup.inputsUnchanged() {
				return setup
			}
			continue
		}
		build := &anonymousSetupBuild{done: make(chan struct{})}
		if anonymousSetupCache.building == nil {
			anonymousSetupCache.building = make(map[anonymousSetupKey]*anonymousSetupBuild)
		}
		anonymousSetupCache.building[key] = build
		anonymousSetupCache.mu.Unlock()
		return buildSharedAnonymousSetup(key, index, build)
	}
}

// buildSharedAnonymousSetup builds the setup for key and wakes its waiters.
// The leader returns its own build for its own call, as a fresh call would; an
// uncacheable build is not handed to waiters, so they rebuild. If the build
// panics, waiters retry and the panic propagates as before.
func buildSharedAnonymousSetup(key anonymousSetupKey, index typesys.Index, build *anonymousSetupBuild) *anonymousSetup {
	defer func() {
		anonymousSetupCache.mu.Lock()
		delete(anonymousSetupCache.building, key)
		anonymousSetupCache.mu.Unlock()
		close(build.done)
	}()
	setup, cacheable := buildAnonymousSetup(index)
	if cacheable {
		anonymousSetupCache.mu.Lock()
		if anonymousSetupCache.entries == nil || len(anonymousSetupCache.entries) >= anonymousSetupCacheLimit {
			anonymousSetupCache.entries = make(map[anonymousSetupKey]*anonymousSetup)
		}
		anonymousSetupCache.entries[key] = setup
		anonymousSetupCache.mu.Unlock()
		build.setup = setup
	}
	return setup
}

// anonymousIndexIdentity names one index value by its backing arrays, so the
// content digest is computed once per index rather than once per row. Indexes
// are immutable after typesys.Build; a caller that edits a built index in
// place must pass fresh slices. The identity holds the array pointers, so an
// array cannot be freed and reused by another index while it is memoized.
type anonymousIndexIdentity struct {
	project               typesys.ProjectInfo
	projectIdentity       [sha256.Size]byte
	types                 anonymousSliceIdentity[typesys.TypeSymbol]
	triggers              anonymousSliceIdentity[typesys.TriggerSymbol]
	objects               anonymousSliceIdentity[schema.Object]
	visualforcePageNames  anonymousSliceIdentity[string]
	visualforcePagesKnown bool
	orgShapeFeatures      anonymousSliceIdentity[string]
	customMetadataRecords anonymousSliceIdentity[schema.CustomMetadataRecord]
	codeIntelSymbols      anonymousSliceIdentity[packageartifact.CodeIntelSymbol]
	codeIntelUses         anonymousSliceIdentity[packageartifact.CodeIntelUse]
	dependencies          anonymousSliceIdentity[typesys.DependencyInfo]
	diagnostics           anonymousSliceIdentity[diagnostic.Diagnostic]
	apiVersion            string
}

type anonymousSliceIdentity[T any] struct {
	first    *T
	len, cap int
}

func newAnonymousSliceIdentity[T any](values []T) anonymousSliceIdentity[T] {
	identity := anonymousSliceIdentity[T]{len: len(values), cap: cap(values)}
	if cap(values) > 0 {
		identity.first = &values[:1][0]
	}
	return identity
}

func anonymousSetupKeyFor(index typesys.Index, apiVersion string) (anonymousSetupKey, bool) {
	projectIdentity, _ := typesys.ProjectIdentityDigest(index)
	identity := anonymousIndexIdentity{
		project:               index.Project,
		projectIdentity:       projectIdentity,
		types:                 newAnonymousSliceIdentity(index.Types),
		triggers:              newAnonymousSliceIdentity(index.Triggers),
		objects:               newAnonymousSliceIdentity(index.Objects),
		visualforcePageNames:  newAnonymousSliceIdentity(index.VisualforcePageNames),
		visualforcePagesKnown: index.VisualforcePagesKnown,
		orgShapeFeatures:      newAnonymousSliceIdentity(index.OrgShapeFeatures),
		customMetadataRecords: newAnonymousSliceIdentity(index.CustomMetadataRecords),
		codeIntelSymbols:      newAnonymousSliceIdentity(index.CodeIntelSymbols),
		codeIntelUses:         newAnonymousSliceIdentity(index.CodeIntelUses),
		dependencies:          newAnonymousSliceIdentity(index.Dependencies),
		diagnostics:           newAnonymousSliceIdentity(index.Diagnostics),
		apiVersion:            apiVersion,
	}
	anonymousSetupCache.mu.Lock()
	key, ok := anonymousSetupCache.keys[identity]
	anonymousSetupCache.mu.Unlock()
	if ok {
		return key, true
	}
	key, ok = newAnonymousSetupKey(index, apiVersion)
	if !ok {
		return key, false
	}
	anonymousSetupCache.mu.Lock()
	if anonymousSetupCache.keys == nil || len(anonymousSetupCache.keys) >= 4*anonymousSetupCacheLimit {
		anonymousSetupCache.keys = make(map[anonymousIndexIdentity]anonymousSetupKey)
	}
	anonymousSetupCache.keys[identity] = key
	anonymousSetupCache.mu.Unlock()
	return key, true
}

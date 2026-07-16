package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// runtimePatchAuthority is attached only to runtimes compiled from a complete,
// immutable Index source snapshot. Disk-restored entries intentionally have no
// authority and therefore cannot become patch bases.
type runtimePatchAuthority struct {
	key                      runtimeCacheKey
	fingerprint              string
	runtimeInputsFingerprint string
	payloadFingerprint       string
	sourceReferences         map[string]string
	transitionApplied        bool
	predecessorKey           runtimeCacheKey
	predecessorFingerprint   string
	affected                 []runtimePatchAffectedOwner
}

type runtimePatchAffectedOwner struct {
	Name      string
	Namespace string
	Path      string
}

// runtimePatchOutcome contains structural facts for callers that plan a wider
// affected-owner closure. Policy and fallback reason strings belong outside
// the runtime transition engine.
type runtimePatchOutcome struct {
	Applied          bool
	RecompiledOwners []string
	RecompiledPaths  []string
	ReusedOwners     []string
	ReusedPaths      []string
	Key              runtimeCacheKey
	EntryValid       bool
	TemplateValid    bool
}

func newRuntimePatchAuthority(index typesys.Index, key runtimeCacheKey, digests *typesys.SourceDigestSet, sources *sourceCache, entry runtimeCacheEntry, org storage.OrgState) *runtimePatchAuthority {
	if digests == nil || !runtimePatchDigestSetsEqual(index, digests.Digest) {
		return nil
	}
	runtimeInputsFingerprint, ok := runtimePatchRuntimeInputsFingerprint(org, entry.PageNames, entry.Methods)
	if !ok {
		return nil
	}
	payloadFingerprint, ok := runtimePatchCompiledPayloadFingerprint(entry)
	if !ok {
		return nil
	}
	return runtimePatchAuthorityFromRetainedSources(index, key, sources, runtimeInputsFingerprint, payloadFingerprint)
}

func runtimePatchAuthorityFromRetainedSources(index typesys.Index, key runtimeCacheKey, sources *sourceCache, runtimeInputsFingerprint, payloadFingerprint string) *runtimePatchAuthority {
	fingerprint, ok := runtimePatchIndexFingerprint(index)
	if !ok || runtimeInputsFingerprint == "" || payloadFingerprint == "" {
		return nil
	}
	digests, ok := runtimePatchIndexDigests(index)
	if !ok {
		return nil
	}
	references := make(map[string]string, len(digests))
	for path, expected := range digests {
		source, retained := sources.retainedRawSource(path)
		if !retained || sha256.Sum256([]byte(source)) != expected {
			return nil
		}
		references[path] = runtimePatchStaticReferenceFingerprint(source)
	}
	return &runtimePatchAuthority{key: key, fingerprint: fingerprint, runtimeInputsFingerprint: runtimeInputsFingerprint, payloadFingerprint: payloadFingerprint, sourceReferences: references}
}

func runtimePatchAuthorityFromTransition(index typesys.Index, key, predecessorKey runtimeCacheKey, predecessorFingerprint, runtimeInputsFingerprint string, previous *runtimePatchAuthority, affected []runtimePatchAffectedOwner, sources *sourceCache, entry runtimeCacheEntry) *runtimePatchAuthority {
	if previous == nil {
		return nil
	}
	fingerprint, ok := runtimePatchIndexFingerprint(index)
	if !ok {
		return nil
	}
	payloadFingerprint, ok := runtimePatchCompiledPayloadFingerprint(entry)
	if !ok {
		return nil
	}
	references := make(map[string]string, len(previous.sourceReferences))
	for path, reference := range previous.sourceReferences {
		references[path] = reference
	}
	for _, owner := range affected {
		source, retained := sources.retainedRawSource(owner.Path)
		expected, expectedOK := index.SourceDigest(owner.Path)
		if !retained || !expectedOK || sha256.Sum256([]byte(source)) != expected {
			return nil
		}
		references[owner.Path] = runtimePatchStaticReferenceFingerprint(source)
	}
	return &runtimePatchAuthority{
		key:                      key,
		fingerprint:              fingerprint,
		runtimeInputsFingerprint: runtimeInputsFingerprint,
		payloadFingerprint:       payloadFingerprint,
		sourceReferences:         references,
		transitionApplied:        true,
		predecessorKey:           predecessorKey,
		predecessorFingerprint:   predecessorFingerprint,
		affected:                 append([]runtimePatchAffectedOwner(nil), affected...),
	}
}

type runtimePatchPayload struct {
	Methods       map[string]vm.Method `json:"methods"`
	Classes       []vm.Class           `json:"classes"`
	Triggers      []vm.Trigger         `json:"triggers"`
	TriggerErrors []string             `json:"triggerErrors"`
	PageNames     []string             `json:"pageNames"`
	BaseError     string               `json:"baseError"`
}

type runtimePatchValueFingerprint struct {
	Kind        vm.ValueKind                       `json:"kind"`
	Int         int64                              `json:"int"`
	Decimal     float64                            `json:"decimal"`
	Bool        bool                               `json:"bool"`
	Text        string                             `json:"text"`
	Type        string                             `json:"type"`
	Static      string                             `json:"static"`
	Runtime     string                             `json:"runtime"`
	Ref         uint64                             `json:"ref"`
	FieldsNil   bool                               `json:"fieldsNil"`
	Fields      []runtimePatchValueFingerprintPair `json:"fields"`
	ListNil     bool                               `json:"listNil"`
	List        []runtimePatchValueFingerprint     `json:"list"`
	SetNil      bool                               `json:"setNil"`
	Set         []runtimePatchValueFingerprint     `json:"set"`
	MapNil      bool                               `json:"mapNil"`
	Map         []runtimePatchValueFingerprintPair `json:"map"`
	MapKeysNil  bool                               `json:"mapKeysNil"`
	MapKeys     []runtimePatchValueFingerprintPair `json:"mapKeys"`
	MapOrderNil bool                               `json:"mapOrderNil"`
	MapOrder    []string                           `json:"mapOrder"`
}

type runtimePatchValueFingerprintPair struct {
	Key   string                       `json:"key"`
	Value runtimePatchValueFingerprint `json:"value"`
}

type runtimePatchFieldValueFingerprint struct {
	ClassIndex   int                          `json:"classIndex"`
	Static       bool                         `json:"static"`
	Name         string                       `json:"name"`
	Value        runtimePatchValueFingerprint `json:"value"`
	InitialValue runtimePatchValueFingerprint `json:"initialValue"`
}

type runtimePatchValueContainerIdentity struct {
	kind byte
	ptr  uintptr
}

func runtimePatchCompiledPayloadFingerprint(entry runtimeCacheEntry) (string, bool) {
	triggerErrors := make([]string, len(entry.TriggerErrors))
	for i, err := range entry.TriggerErrors {
		triggerErrors[i] = runtimePatchErrorIdentity(err)
	}
	payloadBytes, err := json.Marshal(runtimePatchPayload{
		Methods:       entry.Methods,
		Classes:       entry.Classes,
		Triggers:      entry.Triggers,
		TriggerErrors: triggerErrors,
		PageNames:     entry.PageNames,
		BaseError:     runtimePatchErrorIdentity(entry.BaseErr),
	})
	if err != nil {
		return "", false
	}
	values, ok := runtimePatchClassValueFingerprints(entry.Classes)
	if !ok {
		return "", false
	}
	valueBytes, err := json.Marshal(values)
	if err != nil {
		return "", false
	}
	h := sha256.New()
	_, _ = h.Write(payloadBytes)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(valueBytes)
	return hex.EncodeToString(h.Sum(nil)), true
}

func runtimePatchErrorIdentity(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T:%s", err, err.Error())
}

func runtimePatchClassValueFingerprints(classes []vm.Class) ([]runtimePatchFieldValueFingerprint, bool) {
	var out []runtimePatchFieldValueFingerprint
	for classIndex, class := range classes {
		for _, fields := range []struct {
			static bool
			values map[string]vm.Field
		}{{values: class.Fields}, {static: true, values: class.StaticFields}} {
			names := make([]string, 0, len(fields.values))
			for name := range fields.values {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				field := fields.values[name]
				active := make(map[runtimePatchValueContainerIdentity]bool)
				value, ok := runtimePatchValuePayload(field.Value, active, 0)
				if !ok {
					return nil, false
				}
				initialValue, ok := runtimePatchValuePayload(field.InitialValue, active, 0)
				if !ok {
					return nil, false
				}
				out = append(out, runtimePatchFieldValueFingerprint{
					ClassIndex: classIndex, Static: fields.static, Name: name,
					Value: value, InitialValue: initialValue,
				})
			}
		}
	}
	return out, true
}

func runtimePatchValuePayload(value vm.Value, active map[runtimePatchValueContainerIdentity]bool, depth int) (runtimePatchValueFingerprint, bool) {
	if depth > 256 {
		return runtimePatchValueFingerprint{}, false
	}
	out := runtimePatchValueFingerprint{
		Kind: value.Kind, Int: value.Int, Decimal: value.Decimal, Bool: value.Bool,
		Text: value.Text, Type: value.Type, Static: value.Static, Runtime: value.Runtime, Ref: value.Ref,
		FieldsNil: value.Fields == nil, ListNil: value.List == nil, SetNil: value.Set == nil,
		MapNil: value.Map == nil, MapKeysNil: value.MapKeys == nil, MapOrderNil: value.MapOrder == nil,
		MapOrder: append([]string(nil), value.MapOrder...),
	}
	var ok bool
	if out.Fields, ok = runtimePatchValueMapPayload('f', value.Fields, active, depth+1); !ok {
		return runtimePatchValueFingerprint{}, false
	}
	if out.List, ok = runtimePatchValueSlicePayload('l', value.List, active, depth+1); !ok {
		return runtimePatchValueFingerprint{}, false
	}
	if out.Set, ok = runtimePatchValueSlicePayload('s', value.Set, active, depth+1); !ok {
		return runtimePatchValueFingerprint{}, false
	}
	if out.Map, ok = runtimePatchValueMapPayload('m', value.Map, active, depth+1); !ok {
		return runtimePatchValueFingerprint{}, false
	}
	if out.MapKeys, ok = runtimePatchValueMapPayload('k', value.MapKeys, active, depth+1); !ok {
		return runtimePatchValueFingerprint{}, false
	}
	return out, true
}

func runtimePatchValueMapPayload(kind byte, values map[string]vm.Value, active map[runtimePatchValueContainerIdentity]bool, depth int) ([]runtimePatchValueFingerprintPair, bool) {
	if values == nil {
		return nil, true
	}
	identity := runtimePatchValueContainerIdentity{kind: kind, ptr: reflect.ValueOf(values).Pointer()}
	if active[identity] {
		return nil, false
	}
	active[identity] = true
	defer delete(active, identity)
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]runtimePatchValueFingerprintPair, 0, len(names))
	for _, name := range names {
		value, ok := runtimePatchValuePayload(values[name], active, depth)
		if !ok {
			return nil, false
		}
		out = append(out, runtimePatchValueFingerprintPair{Key: name, Value: value})
	}
	return out, true
}

func runtimePatchValueSlicePayload(kind byte, values []vm.Value, active map[runtimePatchValueContainerIdentity]bool, depth int) ([]runtimePatchValueFingerprint, bool) {
	if values == nil {
		return nil, true
	}
	identity := runtimePatchValueContainerIdentity{kind: kind, ptr: reflect.ValueOf(values).Pointer()}
	if active[identity] {
		return nil, false
	}
	active[identity] = true
	defer delete(active, identity)
	out := make([]runtimePatchValueFingerprint, len(values))
	for i, value := range values {
		fingerprint, ok := runtimePatchValuePayload(value, active, depth)
		if !ok {
			return nil, false
		}
		out[i] = fingerprint
	}
	return out, true
}

func runtimePatchAuthorityMatchesPayload(entry runtimeCacheEntry) bool {
	if entry.patchAuthority == nil || entry.patchAuthority.payloadFingerprint == "" {
		return false
	}
	fingerprint, ok := runtimePatchCompiledPayloadFingerprint(entry)
	return ok && fingerprint == entry.patchAuthority.payloadFingerprint
}

func runtimeFromIndexWithSourceDigestsAfter(previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool) (runtimeCacheKey, runtimeCacheEntry, error) {
	affected, _ := runtimePatchOneModifiedOwner(previous, current)
	key, entry, _, err := runtimeFromIndexTransition(previous, current, digests, sources, diskCacheAllowed, nil, affected)
	return key, entry, err
}

func runtimeFromIndexWithSourceDigestsAfterAndPerf(previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters) (runtimeCacheKey, runtimeCacheEntry, error) {
	planningStarted := time.Now()
	affected, _ := runtimePatchOneModifiedOwner(previous, current)
	counters.phases.runtimeKeyNS.Add(time.Since(planningStarted).Nanoseconds())
	key, entry, _, err := runtimeFromIndexTransition(previous, current, digests, sources, diskCacheAllowed, counters, affected)
	return key, entry, err
}

func runtimeFromIndexTransition(previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters, affected []runtimePatchAffectedOwner) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, error) {
	digestCheckStarted := time.Now()
	suppliedDigestsValid := digests == nil || runtimePatchDigestSetsEqual(current, digests.Digest)
	if counters != nil {
		counters.phases.runtimeKeyNS.Add(time.Since(digestCheckStarted).Nanoseconds())
	}
	if !suppliedDigestsValid {
		keyStarted := time.Now()
		key := runtimeKeyWithDigestLookup(current, current.SourceDigest, os.ReadFile)
		if counters != nil {
			counters.phases.runtimeKeyNS.Add(time.Since(keyStarted).Nanoseconds())
		}
		return key, runtimeCacheEntry{}, runtimePatchOutcome{Key: key}, fmt.Errorf("supplied source digests do not match current immutable index")
	}
	var key runtimeCacheKey
	var entry runtimeCacheEntry
	var outcome runtimePatchOutcome
	var applied bool
	key, entry, outcome, applied = tryRuntimePatchTransition(previous, current, sources, counters, affected)
	if applied {
		return key, entry, outcome, nil
	}
	var err error
	if counters == nil {
		key, entry, err = runtimeFromIndexWithSourceDigests(current, digests, sources, diskCacheAllowed)
	} else {
		key, entry, err = runtimeFromIndexWithSourceDigestsAndPerf(current, digests, sources, diskCacheAllowed, counters)
	}
	if err != nil {
		return key, entry, runtimePatchOutcome{Key: key}, err
	}
	outcome = runtimePatchFullBuildOutcome(current, key, entry)
	return key, entry, outcome, err
}

func tryRuntimePatchTransition(previous, current typesys.Index, sources *sourceCache, counters *runPerfCounters, affected []runtimePatchAffectedOwner) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, bool) {
	started := time.Now()
	currentFingerprint, currentOK := runtimePatchIndexFingerprint(current)
	previousFingerprint, previousOK := runtimePatchIndexFingerprint(previous)
	currentKey := runtimeKeyWithDigestLookup(current, current.SourceDigest, os.ReadFile)
	previousKey := runtimeKeyWithDigestLookup(previous, previous.SourceDigest, os.ReadFile)
	if counters != nil {
		counters.phases.runtimeKeyNS.Add(time.Since(started).Nanoseconds())
	}
	if !currentOK || !previousOK || len(affected) == 0 || !runtimePatchTransitionShapeSafe(previous, current, affected) {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	if sources == nil {
		sources = newSourceCache()
	}
	sources.configureNamespaceRemaps(current.Types, current.Triggers)
	sources.bindSourceDigestLookup(current.SourceDigest)
	ambientStarted := time.Now()
	currentOrg := orgFromIndex(current, sources)
	currentPageNames := visualforcePageNames(current)
	currentAmbientFingerprint, ambientOK := runtimePatchAmbientFingerprint(currentOrg, currentPageNames)
	if counters != nil {
		counters.phases.orgBuildNS.Add(time.Since(ambientStarted).Nanoseconds())
	}
	if !ambientOK || sources.sourceSnapshotError() != nil {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	if cached, ok := validMemoryRuntimeEntry(currentKey); ok {
		currentAPIVersionFingerprint, apiVersionsOK := runtimePatchLiveAPIVersionFingerprint(cached.Methods, sources)
		currentRuntimeInputsFingerprint := runtimePatchRuntimeInputsFingerprintFromParts(currentAmbientFingerprint, currentAPIVersionFingerprint)
		if apiVersionsOK {
			if outcome, trusted := runtimePatchTrustedCacheOutcome(current, currentKey, currentFingerprint, currentRuntimeInputsFingerprint, previousKey, previousFingerprint, affected, cached); trusted {
				if counters != nil {
					counters.phases.memoryCacheHits.Add(1)
				}
				return currentKey, cached, outcome, true
			}
		}
		runtimePatchEvictUntrustedCurrentEntry(current, currentKey, currentFingerprint, currentRuntimeInputsFingerprint, previousKey, previousFingerprint, affected)
	}
	previousEntry, ok := validMemoryRuntimeEntry(previousKey)
	if !ok {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	currentAPIVersionFingerprint, apiVersionsOK := runtimePatchLiveAPIVersionFingerprint(previousEntry.Methods, sources)
	if !apiVersionsOK {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	currentRuntimeInputsFingerprint := runtimePatchRuntimeInputsFingerprintFromParts(currentAmbientFingerprint, currentAPIVersionFingerprint)
	if !runtimePatchBaseEntryTrusted(previousEntry, previousKey, previousFingerprint, currentRuntimeInputsFingerprint) {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	if !runtimePatchAffectedClosureValid(current, affected) {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	sourceStarted := time.Now()
	sourcesValid := runtimePatchValidateAffectedSources(previous, current, previousEntry.patchAuthority, affected, sources)
	if counters != nil {
		counters.phases.projectCompileNS.Add(time.Since(sourceStarted).Nanoseconds())
	}
	if !sourcesValid {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}

	include := runtimePatchOwnerSelector(affected)
	compileStarted := time.Now()
	changedMethods := compileProjectMethodsWhere(current, include, sources)
	changedClasses := compileProjectClassesWhere(current, changedMethods, include, sources)
	if !runtimePatchCompiledMethodsReferenceSafe(changedMethods) || !runtimePatchCompiledClassesReferenceSafe(changedClasses) {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	methods := runtimePatchMethods(previousEntry.Methods, changedMethods, affected)
	compiledAPIVersionFingerprint, compiledAPIVersionsOK := runtimePatchCompiledAPIVersionFingerprint(methods)
	if !compiledAPIVersionsOK || compiledAPIVersionFingerprint != currentAPIVersionFingerprint {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	classes, replaced := runtimePatchClasses(previousEntry.Classes, changedClasses, affected)
	if !replaced || sources.sourceSnapshotError() != nil {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	triggers := append([]vm.Trigger(nil), previousEntry.Triggers...)
	triggerErrors := append([]error(nil), previousEntry.TriggerErrors...)
	baseMachine := vm.New(nil)
	baseMachine.SetTraceEnabled(false)
	registerVisualforcePages(baseMachine, currentPageNames)
	baseErr := registerBaseRuntime(baseMachine, methods, classes, triggers)
	if counters != nil {
		counters.phases.projectCompileNS.Add(time.Since(compileStarted).Nanoseconds())
	}
	if baseErr != nil || sources.sourceSnapshotError() != nil {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}

	orgStarted := time.Now()
	restored := vm.NewRestoredRuntimeTemplate(currentOrg, baseMachine)
	if counters != nil {
		counters.phases.orgBuildNS.Add(time.Since(orgStarted).Nanoseconds())
	}
	entry := runtimeCacheEntry{
		Methods:       methods,
		Classes:       classes,
		Triggers:      triggers,
		TriggerErrors: triggerErrors,
		PageNames:     append([]string(nil), currentPageNames...),
		BaseErr:       nil,
		restored:      restored,
	}
	authorityStarted := time.Now()
	patchAuthority := runtimePatchAuthorityFromTransition(current, currentKey, previousKey, previousFingerprint, currentRuntimeInputsFingerprint, previousEntry.patchAuthority, affected, sources, entry)
	if counters != nil {
		counters.phases.projectCompileNS.Add(time.Since(authorityStarted).Nanoseconds())
	}
	entry.patchAuthority = patchAuthority
	if !entry.restored.Valid() || entry.patchAuthority == nil {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	clonedEntry, cloneOK := cloneRuntimeCacheEntryChecked(entry)
	if !cloneOK || !clonedEntry.restored.Valid() {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}

	runtimeCacheMu.Lock()
	base, baseOK := runtimeCache[previousKey]
	baseAuthorityOK := baseOK && runtimePatchBaseEntryTrusted(base, previousKey, previousFingerprint, currentRuntimeInputsFingerprint)
	if !baseAuthorityOK {
		runtimeCacheMu.Unlock()
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	if published, exists := runtimeCache[currentKey]; exists && published.restored.Valid() {
		if outcome, trusted := runtimePatchTrustedCacheOutcome(current, currentKey, currentFingerprint, currentRuntimeInputsFingerprint, previousKey, previousFingerprint, affected, published); trusted {
			cloned, clonedOK := cloneRuntimeCacheEntryChecked(published)
			if clonedOK && cloned.restored.Valid() {
				runtimeCacheMu.Unlock()
				if counters != nil {
					counters.phases.memoryCacheHits.Add(1)
				}
				return currentKey, cloned, outcome, true
			}
		}
		delete(runtimeCache, currentKey)
	}
	runtimeCache[currentKey] = entry
	runtimeCacheMu.Unlock()
	if counters != nil {
		counters.phases.cacheMisses.Add(1)
	}
	return currentKey, clonedEntry, runtimePatchAppliedOutcome(current, affected, currentKey, clonedEntry), true
}

func runtimePatchBaseEntryTrusted(entry runtimeCacheEntry, key runtimeCacheKey, fingerprint, runtimeInputsFingerprint string) bool {
	return entry.restored.Valid() && entry.patchAuthority != nil &&
		entry.patchAuthority.key == key &&
		entry.patchAuthority.fingerprint == fingerprint &&
		entry.patchAuthority.runtimeInputsFingerprint == runtimeInputsFingerprint &&
		runtimePatchAuthorityMatchesPayload(entry) &&
		entry.BaseErr == nil && len(entry.TriggerErrors) == 0
}

func runtimePatchValidateAffectedSources(previous, current typesys.Index, previousAuthority *runtimePatchAuthority, affected []runtimePatchAffectedOwner, sources *sourceCache) bool {
	if previousAuthority == nil {
		return false
	}
	for _, owner := range affected {
		if _, err := sources.read(owner.Path); err != nil {
			return false
		}
		previousDigest, previousDigestOK := previous.SourceDigest(owner.Path)
		currentDigest, currentDigestOK := current.SourceDigest(owner.Path)
		if !previousDigestOK || !currentDigestOK {
			return false
		}
		if previousDigest != currentDigest {
			source, retained := sources.retainedRawSource(owner.Path)
			if !retained || sha256.Sum256([]byte(source)) != currentDigest || runtimePatchHasDynamicReference(source) || previousAuthority.sourceReferences[owner.Path] != runtimePatchStaticReferenceFingerprint(source) {
				return false
			}
		}
	}
	return sources.sourceSnapshotError() == nil
}

func runtimePatchTrustedCacheOutcome(index typesys.Index, key runtimeCacheKey, fingerprint, runtimeInputsFingerprint string, predecessorKey runtimeCacheKey, predecessorFingerprint string, affected []runtimePatchAffectedOwner, entry runtimeCacheEntry) (runtimePatchOutcome, bool) {
	authority := entry.patchAuthority
	if authority == nil || authority.key != key || authority.fingerprint != fingerprint || authority.runtimeInputsFingerprint != runtimeInputsFingerprint {
		return runtimePatchOutcome{}, false
	}
	if !runtimePatchAuthorityMatchesPayload(entry) {
		return runtimePatchOutcome{}, false
	}
	if authority.transitionApplied {
		if authority.predecessorKey != predecessorKey || authority.predecessorFingerprint != predecessorFingerprint || !runtimePatchAffectedOwnersEqual(authority.affected, affected) {
			return runtimePatchOutcome{}, false
		}
		return runtimePatchAppliedOutcome(index, authority.affected, key, entry), true
	}
	return runtimePatchFullBuildOutcome(index, key, entry), true
}

func runtimePatchEvictUntrustedCurrentEntry(index typesys.Index, key runtimeCacheKey, fingerprint, runtimeInputsFingerprint string, predecessorKey runtimeCacheKey, predecessorFingerprint string, affected []runtimePatchAffectedOwner) {
	runtimeCacheMu.Lock()
	entry, ok := runtimeCache[key]
	if ok {
		if _, trusted := runtimePatchTrustedCacheOutcome(index, key, fingerprint, runtimeInputsFingerprint, predecessorKey, predecessorFingerprint, affected, entry); !trusted {
			delete(runtimeCache, key)
		}
	}
	runtimeCacheMu.Unlock()
}

func runtimePatchAffectedOwnersEqual(left, right []runtimePatchAffectedOwner) bool {
	if len(left) != len(right) {
		return false
	}
	want := make(map[string]bool, len(left))
	for _, owner := range left {
		want[runtimePatchOwnerKey(owner)] = true
	}
	if len(want) != len(left) {
		return false
	}
	got := make(map[string]bool, len(right))
	for _, owner := range right {
		key := runtimePatchOwnerKey(owner)
		if !want[key] || got[key] {
			return false
		}
		got[key] = true
	}
	return len(got) == len(want)
}

func runtimePatchOneModifiedOwner(previous, current typesys.Index) ([]runtimePatchAffectedOwner, bool) {
	previousDigests, previousOK := runtimePatchIndexDigests(previous)
	currentDigests, currentOK := runtimePatchIndexDigests(current)
	if !previousOK || !currentOK || len(previousDigests) != len(currentDigests) {
		return nil, false
	}
	changedPath := ""
	for path, previousDigest := range previousDigests {
		currentDigest, exists := currentDigests[path]
		if !exists {
			return nil, false
		}
		if currentDigest != previousDigest {
			if changedPath != "" {
				return nil, false
			}
			changedPath = path
		}
	}
	if changedPath == "" || !runtimePatchStaticSurfacesEqual(previous, current, map[string]bool{changedPath: true}) {
		return nil, false
	}
	var owner runtimePatchAffectedOwner
	owners := 0
	for i := range current.Types {
		if current.Types[i].File != changedPath {
			continue
		}
		typ := current.Types[i]
		if typ.Kind != apexast.DeclarationClass || typ.Dependency || typ.Artifact || strings.TrimSpace(typ.Name) == "" {
			return nil, false
		}
		owner = runtimePatchAffectedOwner{Name: typ.Name, Namespace: typ.Namespace, Path: typ.File}
		owners++
	}
	if owners != 1 {
		return nil, false
	}
	return []runtimePatchAffectedOwner{owner}, true
}

func runtimePatchTransitionShapeSafe(previous, current typesys.Index, affected []runtimePatchAffectedOwner) bool {
	previousDigests, previousOK := runtimePatchIndexDigests(previous)
	currentDigests, currentOK := runtimePatchIndexDigests(current)
	if !previousOK || !currentOK || len(previousDigests) != len(currentDigests) {
		return false
	}
	affectedPaths := make(map[string]bool, len(affected))
	for _, owner := range affected {
		affectedPaths[owner.Path] = true
	}
	changedPaths := make(map[string]bool)
	for path, previousDigest := range previousDigests {
		currentDigest, exists := currentDigests[path]
		if !exists {
			return false
		}
		if currentDigest != previousDigest {
			if !affectedPaths[path] {
				return false
			}
			changedPaths[path] = true
		}
	}
	return len(changedPaths) > 0 && runtimePatchStaticSurfacesEqual(previous, current, changedPaths)
}

func runtimePatchStaticSurfacesEqual(previous, current typesys.Index, changedPaths map[string]bool) bool {
	if previous.Project != current.Project || !typesys.SameProjectIdentity(previous, current) ||
		!reflect.DeepEqual(previous.Objects, current.Objects) ||
		!reflect.DeepEqual(previous.CustomMetadataRecords, current.CustomMetadataRecords) ||
		!reflect.DeepEqual(previous.Dependencies, current.Dependencies) ||
		!reflect.DeepEqual(previous.Diagnostics, current.Diagnostics) ||
		!reflect.DeepEqual(previous.Triggers, current.Triggers) ||
		len(previous.Types) != len(current.Types) {
		return false
	}
	for i := range previous.Types {
		if changedPaths[previous.Types[i].File] || changedPaths[current.Types[i].File] {
			if !reflect.DeepEqual(runtimePatchTypeShape(previous.Types[i]), runtimePatchTypeShape(current.Types[i])) {
				return false
			}
			continue
		}
		if !reflect.DeepEqual(previous.Types[i], current.Types[i]) {
			return false
		}
	}
	return reflect.DeepEqual(runtimePatchCodeIntelSymbols(previous.CodeIntelSymbols, changedPaths), runtimePatchCodeIntelSymbols(current.CodeIntelSymbols, changedPaths)) &&
		reflect.DeepEqual(runtimePatchCodeIntelUses(previous.CodeIntelUses, changedPaths), runtimePatchCodeIntelUses(current.CodeIntelUses, changedPaths))
}

func runtimePatchTypeShape(typ typesys.TypeSymbol) typesys.TypeSymbol {
	typ.Range = diagnostic.Range{}
	typ.Members = append([]typesys.MemberSymbol(nil), typ.Members...)
	for i := range typ.Members {
		typ.Members[i].Range = diagnostic.Range{}
		typ.Members[i].Parameters = append([]apexast.Parameter(nil), typ.Members[i].Parameters...)
		for j := range typ.Members[i].Parameters {
			typ.Members[i].Parameters[j].Range = diagnostic.Range{}
		}
		typ.Members[i].Accessors = append([]apexast.Accessor(nil), typ.Members[i].Accessors...)
		for j := range typ.Members[i].Accessors {
			typ.Members[i].Accessors[j].Range = diagnostic.Range{}
		}
	}
	return typ
}

func runtimePatchCodeIntelSymbols(in []packageartifact.CodeIntelSymbol, changedPaths map[string]bool) []packageartifact.CodeIntelSymbol {
	out := append([]packageartifact.CodeIntelSymbol(nil), in...)
	for i := range out {
		if changedPaths[out[i].File] {
			out[i].Range = diagnostic.Range{}
		}
	}
	return out
}

func runtimePatchCodeIntelUses(in []packageartifact.CodeIntelUse, changedPaths map[string]bool) []packageartifact.CodeIntelUse {
	out := append([]packageartifact.CodeIntelUse(nil), in...)
	for i := range out {
		if changedPaths[out[i].File] {
			out[i].Range = diagnostic.Range{}
		}
	}
	return out
}

func runtimePatchHasDynamicReference(source string) bool {
	compact := strings.ToLower(strings.Join(strings.Fields(runtimePatchStripComments(source)), ""))
	return strings.Contains(compact, "type.forname(") ||
		strings.Contains(compact, "schema.getglobaldescribe(") ||
		strings.Contains(compact, "json.deserialize(") ||
		strings.Contains(compact, "json.deserializeuntyped(") ||
		strings.Contains(compact, "database.query(") ||
		strings.Contains(compact, "database.querywithbinds(") ||
		strings.Contains(compact, "database.countquery(") ||
		strings.Contains(compact, "database.countquerywithbinds(") ||
		strings.Contains(compact, "database.getquerylocator(") ||
		strings.Contains(compact, "database.getquerylocatorwithbinds(") ||
		strings.Contains(compact, "search.query(")
}

func runtimePatchCompiledMethodsReferenceSafe(methods map[string]vm.Method) bool {
	for _, method := range methods {
		if !runtimePatchProgramReferenceSafe(method.Program) {
			return false
		}
	}
	return true
}

func runtimePatchCompiledClassesReferenceSafe(classes []vm.Class) bool {
	for _, class := range classes {
		if !runtimePatchCompiledMethodsReferenceSafe(class.Methods) ||
			!runtimePatchMethodSliceReferenceSafe(class.Constructors) ||
			!runtimePatchMethodSliceReferenceSafe(class.StaticInitializers) ||
			!runtimePatchMethodSliceReferenceSafe(class.InstanceInitializers) ||
			!runtimePatchFieldAccessorsReferenceSafe(class.Fields) ||
			!runtimePatchFieldAccessorsReferenceSafe(class.StaticFields) {
			return false
		}
	}
	return true
}

func runtimePatchMethodSliceReferenceSafe(methods []vm.Method) bool {
	for _, method := range methods {
		if !runtimePatchProgramReferenceSafe(method.Program) {
			return false
		}
	}
	return true
}

func runtimePatchFieldAccessorsReferenceSafe(fields map[string]vm.Field) bool {
	for _, field := range fields {
		if field.Getter != nil && !runtimePatchProgramReferenceSafe(field.Getter.Program) {
			return false
		}
		if field.Setter != nil && !runtimePatchProgramReferenceSafe(field.Setter.Program) {
			return false
		}
	}
	return true
}

// Runtime patching is limited to expression-only local computation. Calls,
// field access, SOQL, and DML can resolve types or data through values that a
// source-token fingerprint cannot prove stable.
func runtimePatchProgramReferenceSafe(program ir.Program) bool {
	return runtimePatchInstructionsReferenceSafe(program.Instructions)
}

func runtimePatchInstructionsReferenceSafe(instructions []ir.Instruction) bool {
	for _, instruction := range instructions {
		if instruction.Op == ir.OpDML || instruction.Field != "" || !runtimePatchExprReferenceSafe(instruction.Expr) {
			return false
		}
		if instruction.Init != nil && !runtimePatchInstructionsReferenceSafe([]ir.Instruction{*instruction.Init}) {
			return false
		}
		if instruction.Update != nil && !runtimePatchInstructionsReferenceSafe([]ir.Instruction{*instruction.Update}) {
			return false
		}
		if !runtimePatchInstructionsReferenceSafe(instruction.Inits) ||
			!runtimePatchInstructionsReferenceSafe(instruction.Updates) ||
			!runtimePatchInstructionsReferenceSafe(instruction.Then) ||
			!runtimePatchInstructionsReferenceSafe(instruction.Else) ||
			!runtimePatchInstructionsReferenceSafe(instruction.Catch) ||
			!runtimePatchInstructionsReferenceSafe(instruction.Finally) {
			return false
		}
		for _, clause := range instruction.Catches {
			if !runtimePatchInstructionsReferenceSafe(clause.Body) {
				return false
			}
		}
		for _, switchCase := range instruction.Cases {
			for _, expr := range switchCase.Exprs {
				if !runtimePatchExprReferenceSafe(expr) {
					return false
				}
			}
			if !runtimePatchInstructionsReferenceSafe(switchCase.Body) {
				return false
			}
		}
	}
	return true
}

func runtimePatchExprReferenceSafe(expr ir.Expr) bool {
	if expr.Kind == ir.ExprCall || expr.Kind == ir.ExprSOQL {
		return false
	}
	if expr.Left != nil && !runtimePatchExprReferenceSafe(*expr.Left) {
		return false
	}
	if expr.Right != nil && !runtimePatchExprReferenceSafe(*expr.Right) {
		return false
	}
	for _, arg := range expr.Args {
		if !runtimePatchExprReferenceSafe(arg) {
			return false
		}
	}
	for _, arg := range expr.NamedArgs {
		if !runtimePatchExprReferenceSafe(arg.Expr) {
			return false
		}
	}
	return true
}

var runtimePatchReferencePattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

func runtimePatchStaticReferenceFingerprint(source string) string {
	withoutComments := runtimePatchStripComments(source)
	references := runtimePatchReferencePattern.FindAllString(withoutComments, -1)
	for i := range references {
		references[i] = strings.Join(strings.Fields(references[i]), "")
	}
	return strings.Join(references, "\x00") + "\x01" + strings.Join(runtimePatchStringLiterals(withoutComments), "\x00")
}

func runtimePatchStripComments(source string) string {
	var out strings.Builder
	out.Grow(len(source))
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'' || source[i] == '"':
			quote := source[i]
			out.WriteByte(source[i])
			i++
			for i < len(source) {
				out.WriteByte(source[i])
				if source[i] == '\\' && i+1 < len(source) {
					i++
					out.WriteByte(source[i])
					i++
					continue
				}
				if source[i] == quote {
					i++
					break
				}
				i++
			}
		case i+1 < len(source) && source[i:i+2] == "//":
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case i+1 < len(source) && source[i:i+2] == "/*":
			i += 2
			for i+1 < len(source) && source[i:i+2] != "*/" {
				if source[i] == '\n' {
					out.WriteByte('\n')
				}
				i++
			}
			if i+1 < len(source) {
				i += 2
			}
		default:
			out.WriteByte(source[i])
			i++
		}
	}
	return out.String()
}

func runtimePatchStringLiterals(source string) []string {
	var literals []string
	for i := 0; i < len(source); {
		if source[i] != '\'' && source[i] != '"' {
			i++
			continue
		}
		start := i
		quote := source[i]
		i++
		for i < len(source) {
			if source[i] == '\\' && i+1 < len(source) {
				i += 2
				continue
			}
			if source[i] == quote {
				i++
				break
			}
			i++
		}
		literals = append(literals, source[start:i])
	}
	return literals
}

func runtimePatchAffectedClosureValid(index typesys.Index, affected []runtimePatchAffectedOwner) bool {
	seen := make(map[string]bool, len(affected))
	affectedPaths := make(map[string]bool, len(affected))
	for _, owner := range affected {
		key := runtimePatchOwnerKey(owner)
		if owner.Path == "" || owner.Name == "" || seen[key] {
			return false
		}
		seen[key] = true
		affectedPaths[owner.Path] = true
		found := false
		for _, typ := range index.Types {
			if typ.File == owner.Path && typ.Name == owner.Name && typ.Namespace == owner.Namespace && !typ.Dependency && !typ.Artifact {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, typ := range index.Types {
		if !affectedPaths[typ.File] || typ.Dependency || typ.Artifact {
			continue
		}
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface && typ.Kind != apexast.DeclarationEnum {
			continue
		}
		owner := runtimePatchAffectedOwner{Name: typ.Name, Namespace: typ.Namespace, Path: typ.File}
		if !seen[runtimePatchOwnerKey(owner)] {
			return false
		}
	}
	return true
}

func runtimePatchOwnerSelector(affected []runtimePatchAffectedOwner) func(typesys.TypeSymbol) bool {
	owners := make(map[string]bool, len(affected))
	for _, owner := range affected {
		owners[runtimePatchOwnerKey(owner)] = true
	}
	return func(typ typesys.TypeSymbol) bool {
		return owners[runtimePatchOwnerKey(runtimePatchAffectedOwner{Name: typ.Name, Namespace: typ.Namespace, Path: typ.File})]
	}
}

func runtimePatchMethods(previous, changed map[string]vm.Method, affected []runtimePatchAffectedOwner) map[string]vm.Method {
	paths := make(map[string]bool, len(affected))
	for _, owner := range affected {
		paths[owner.Path] = true
	}
	out := make(map[string]vm.Method, len(previous)+len(changed))
	for key, method := range previous {
		if !paths[method.File] {
			out[key] = method
		}
	}
	for key, method := range changed {
		out[key] = method
	}
	return out
}

func runtimePatchClasses(previous, changed []vm.Class, affected []runtimePatchAffectedOwner) ([]vm.Class, bool) {
	replacements := make(map[string]vm.Class, len(changed))
	for _, class := range changed {
		replacements[runtimePatchClassKey(class.Namespace, class.Name)] = class
	}
	want := make(map[string]bool, len(affected))
	for _, owner := range affected {
		want[runtimePatchClassKey(owner.Namespace, owner.Name)] = true
	}
	replaced := make(map[string]bool, len(want))
	out := make([]vm.Class, 0, len(previous))
	for _, class := range previous {
		key := runtimePatchClassKey(class.Namespace, class.Name)
		if want[key] {
			replacement, ok := replacements[key]
			if !ok {
				return nil, false
			}
			cloned, ok := runtimePatchCloneClass(replacement)
			if !ok {
				return nil, false
			}
			out = append(out, cloned)
			replaced[key] = true
			continue
		}
		cloned, ok := runtimePatchCloneClass(class)
		if !ok {
			return nil, false
		}
		out = append(out, cloned)
	}
	return out, len(replaced) == len(want)
}

func runtimePatchCloneClass(in vm.Class) (vm.Class, bool) {
	out := in
	out.Interfaces = runtimePatchCloneSlice(in.Interfaces)
	out.FieldOrder = runtimePatchCloneSlice(in.FieldOrder)
	out.StaticFieldOrder = runtimePatchCloneSlice(in.StaticFieldOrder)
	out.EnumValues = runtimePatchCloneSlice(in.EnumValues)
	out.Modifiers = runtimePatchCloneSlice(in.Modifiers)
	var ok bool
	if out.Fields, ok = runtimePatchCloneFields(in.Fields); !ok {
		return vm.Class{}, false
	}
	if out.StaticFields, ok = runtimePatchCloneFields(in.StaticFields); !ok {
		return vm.Class{}, false
	}
	if out.Methods, ok = runtimePatchCloneMethods(in.Methods); !ok {
		return vm.Class{}, false
	}
	if out.Constructors, ok = runtimePatchCloneMethodSlice(in.Constructors); !ok {
		return vm.Class{}, false
	}
	if out.StaticInitializers, ok = runtimePatchCloneMethodSlice(in.StaticInitializers); !ok {
		return vm.Class{}, false
	}
	if out.InstanceInitializers, ok = runtimePatchCloneMethodSlice(in.InstanceInitializers); !ok {
		return vm.Class{}, false
	}
	return out, true
}

func runtimePatchCloneFields(in map[string]vm.Field) (map[string]vm.Field, bool) {
	if in == nil {
		return nil, true
	}
	out := make(map[string]vm.Field, len(in))
	for key, field := range in {
		field.Modifiers = runtimePatchCloneSlice(field.Modifiers)
		var ok bool
		if field.Value, ok = runtimePatchCloneValue(field.Value, make(map[runtimePatchValueContainerIdentity]bool), 0); !ok {
			return nil, false
		}
		if field.InitialValue, ok = runtimePatchCloneValue(field.InitialValue, make(map[runtimePatchValueContainerIdentity]bool), 0); !ok {
			return nil, false
		}
		if field.Getter != nil {
			getter, ok := runtimePatchCloneMethod(*field.Getter)
			if !ok {
				return nil, false
			}
			field.Getter = &getter
		}
		if field.Setter != nil {
			setter, ok := runtimePatchCloneMethod(*field.Setter)
			if !ok {
				return nil, false
			}
			field.Setter = &setter
		}
		out[key] = field
	}
	return out, true
}

func runtimePatchCloneMethods(in map[string]vm.Method) (map[string]vm.Method, bool) {
	if in == nil {
		return nil, true
	}
	out := make(map[string]vm.Method, len(in))
	for key, method := range in {
		cloned, ok := runtimePatchCloneMethod(method)
		if !ok {
			return nil, false
		}
		out[key] = cloned
	}
	return out, true
}

func runtimePatchCloneMethodSlice(in []vm.Method) ([]vm.Method, bool) {
	if in == nil {
		return nil, true
	}
	out := make([]vm.Method, len(in))
	for i, method := range in {
		cloned, ok := runtimePatchCloneMethod(method)
		if !ok {
			return nil, false
		}
		out[i] = cloned
	}
	return out, true
}

func runtimePatchCloneMethod(in vm.Method) (vm.Method, bool) {
	out := in
	out.Params = runtimePatchCloneSlice(in.Params)
	out.Modifiers = runtimePatchCloneSlice(in.Modifiers)
	program, ok := runtimePatchCloneProgram(in.Program, make(map[*ir.Expr]bool), 0)
	if !ok {
		return vm.Method{}, false
	}
	out.Program = program
	return out, true
}

func runtimePatchCloneProgram(in ir.Program, active map[*ir.Expr]bool, depth int) (ir.Program, bool) {
	instructions, ok := runtimePatchCloneInstructions(in.Instructions, active, depth+1)
	if !ok {
		return ir.Program{}, false
	}
	return ir.Program{Instructions: instructions, Source: in.Source}, true
}

func runtimePatchCloneInstructions(in []ir.Instruction, active map[*ir.Expr]bool, depth int) ([]ir.Instruction, bool) {
	if depth > 256 {
		return nil, false
	}
	if in == nil {
		return nil, true
	}
	out := make([]ir.Instruction, len(in))
	for i, instruction := range in {
		cloned, ok := runtimePatchCloneInstruction(instruction, active, depth+1)
		if !ok {
			return nil, false
		}
		out[i] = cloned
	}
	return out, true
}

func runtimePatchCloneInstruction(in ir.Instruction, active map[*ir.Expr]bool, depth int) (ir.Instruction, bool) {
	if depth > 256 {
		return ir.Instruction{}, false
	}
	out := in
	out.CatchTypes = runtimePatchCloneSlice(in.CatchTypes)
	var ok bool
	if out.Expr, ok = runtimePatchCloneExpr(in.Expr, active, depth+1); !ok {
		return ir.Instruction{}, false
	}
	if in.Init != nil {
		init, ok := runtimePatchCloneInstruction(*in.Init, active, depth+1)
		if !ok {
			return ir.Instruction{}, false
		}
		out.Init = &init
	}
	if in.Update != nil {
		update, ok := runtimePatchCloneInstruction(*in.Update, active, depth+1)
		if !ok {
			return ir.Instruction{}, false
		}
		out.Update = &update
	}
	if out.Inits, ok = runtimePatchCloneInstructions(in.Inits, active, depth+1); !ok {
		return ir.Instruction{}, false
	}
	if out.Updates, ok = runtimePatchCloneInstructions(in.Updates, active, depth+1); !ok {
		return ir.Instruction{}, false
	}
	if out.Then, ok = runtimePatchCloneInstructions(in.Then, active, depth+1); !ok {
		return ir.Instruction{}, false
	}
	if out.Else, ok = runtimePatchCloneInstructions(in.Else, active, depth+1); !ok {
		return ir.Instruction{}, false
	}
	if out.Catch, ok = runtimePatchCloneInstructions(in.Catch, active, depth+1); !ok {
		return ir.Instruction{}, false
	}
	if out.Finally, ok = runtimePatchCloneInstructions(in.Finally, active, depth+1); !ok {
		return ir.Instruction{}, false
	}
	if in.Catches != nil {
		out.Catches = make([]ir.CatchClause, len(in.Catches))
	}
	for i, clause := range in.Catches {
		body, ok := runtimePatchCloneInstructions(clause.Body, active, depth+1)
		if !ok {
			return ir.Instruction{}, false
		}
		out.Catches[i] = clause
		out.Catches[i].Types = runtimePatchCloneSlice(clause.Types)
		out.Catches[i].Body = body
	}
	if in.Cases != nil {
		out.Cases = make([]ir.SwitchCase, len(in.Cases))
	}
	for i, switchCase := range in.Cases {
		out.Cases[i] = switchCase
		out.Cases[i].Exprs = make([]ir.Expr, len(switchCase.Exprs))
		for j, expr := range switchCase.Exprs {
			cloned, ok := runtimePatchCloneExpr(expr, active, depth+1)
			if !ok {
				return ir.Instruction{}, false
			}
			out.Cases[i].Exprs[j] = cloned
		}
		body, ok := runtimePatchCloneInstructions(switchCase.Body, active, depth+1)
		if !ok {
			return ir.Instruction{}, false
		}
		out.Cases[i].Body = body
	}
	return out, true
}

func runtimePatchCloneExpr(in ir.Expr, active map[*ir.Expr]bool, depth int) (ir.Expr, bool) {
	out := in
	if in.Args != nil {
		out.Args = make([]ir.Expr, len(in.Args))
	}
	for i, arg := range in.Args {
		cloned, ok := runtimePatchCloneExpr(arg, active, depth+1)
		if !ok {
			return ir.Expr{}, false
		}
		out.Args[i] = cloned
	}
	if in.NamedArgs != nil {
		out.NamedArgs = make([]ir.NamedArg, len(in.NamedArgs))
	}
	for i, arg := range in.NamedArgs {
		cloned, ok := runtimePatchCloneExpr(arg.Expr, active, depth+1)
		if !ok {
			return ir.Expr{}, false
		}
		out.NamedArgs[i] = ir.NamedArg{Name: arg.Name, Expr: cloned}
	}
	for _, pair := range []struct {
		in  *ir.Expr
		out **ir.Expr
	}{{in: in.Left, out: &out.Left}, {in: in.Right, out: &out.Right}} {
		if pair.in == nil {
			*pair.out = nil
			continue
		}
		if active[pair.in] {
			return ir.Expr{}, false
		}
		active[pair.in] = true
		cloned, ok := runtimePatchCloneExpr(*pair.in, active, depth+1)
		delete(active, pair.in)
		if !ok {
			return ir.Expr{}, false
		}
		*pair.out = &cloned
	}
	return out, true
}

func runtimePatchCloneValue(in vm.Value, active map[runtimePatchValueContainerIdentity]bool, depth int) (vm.Value, bool) {
	if depth > 256 {
		return vm.Value{}, false
	}
	out := in
	var ok bool
	if out.Fields, ok = runtimePatchCloneValueMap('f', in.Fields, active, depth+1); !ok {
		return vm.Value{}, false
	}
	if out.List, ok = runtimePatchCloneValueSlice('l', in.List, active, depth+1); !ok {
		return vm.Value{}, false
	}
	if out.Set, ok = runtimePatchCloneValueSlice('s', in.Set, active, depth+1); !ok {
		return vm.Value{}, false
	}
	if out.Map, ok = runtimePatchCloneValueMap('m', in.Map, active, depth+1); !ok {
		return vm.Value{}, false
	}
	if out.MapKeys, ok = runtimePatchCloneValueMap('k', in.MapKeys, active, depth+1); !ok {
		return vm.Value{}, false
	}
	out.MapOrder = runtimePatchCloneSlice(in.MapOrder)
	return out, true
}

func runtimePatchCloneValueMap(kind byte, in map[string]vm.Value, active map[runtimePatchValueContainerIdentity]bool, depth int) (map[string]vm.Value, bool) {
	if in == nil {
		return nil, true
	}
	identity := runtimePatchValueContainerIdentity{kind: kind, ptr: reflect.ValueOf(in).Pointer()}
	if active[identity] {
		return nil, false
	}
	active[identity] = true
	defer delete(active, identity)
	out := make(map[string]vm.Value, len(in))
	for key, value := range in {
		cloned, ok := runtimePatchCloneValue(value, active, depth+1)
		if !ok {
			return nil, false
		}
		out[key] = cloned
	}
	return out, true
}

func runtimePatchCloneValueSlice(kind byte, in []vm.Value, active map[runtimePatchValueContainerIdentity]bool, depth int) ([]vm.Value, bool) {
	if in == nil {
		return nil, true
	}
	identity := runtimePatchValueContainerIdentity{kind: kind, ptr: reflect.ValueOf(in).Pointer()}
	if active[identity] {
		return nil, false
	}
	active[identity] = true
	defer delete(active, identity)
	out := make([]vm.Value, len(in))
	for i, value := range in {
		cloned, ok := runtimePatchCloneValue(value, active, depth+1)
		if !ok {
			return nil, false
		}
		out[i] = cloned
	}
	return out, true
}

func runtimePatchCloneSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func runtimePatchDigestSetsEqual(index typesys.Index, lookup func(string) ([sha256.Size]byte, bool)) bool {
	digests, ok := runtimePatchIndexDigests(index)
	if !ok {
		return false
	}
	for path, digest := range digests {
		other, exists := lookup(path)
		if !exists || other != digest {
			return false
		}
	}
	return true
}

func runtimePatchIndexDigests(index typesys.Index) (map[string][sha256.Size]byte, bool) {
	paths := make(map[string]bool)
	for _, typ := range index.Types {
		if typ.File != "" {
			paths[typ.File] = true
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.File != "" {
			paths[trigger.File] = true
		}
	}
	digests := make(map[string][sha256.Size]byte, len(paths))
	for path := range paths {
		digest, ok := index.SourceDigest(path)
		if !ok {
			return nil, false
		}
		digests[path] = digest
	}
	return digests, true
}

func runtimePatchIndexFingerprint(index typesys.Index) (string, bool) {
	digests, ok := runtimePatchIndexDigests(index)
	if !ok {
		return "", false
	}
	projectIdentity, ok := typesys.ProjectIdentityDigest(index)
	if !ok {
		return "", false
	}
	encoded, err := json.Marshal(index)
	if err != nil {
		return "", false
	}
	paths := make([]string, 0, len(digests))
	for path := range digests {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha256.New()
	_, _ = h.Write(encoded)
	_, _ = h.Write(projectIdentity[:])
	for _, path := range paths {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(path))
		digest := digests[path]
		_, _ = h.Write(digest[:])
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func runtimePatchAmbientFingerprint(org storage.OrgState, pageNames []string) (string, bool) {
	ambientOrg := org.Clone()
	delete(ambientOrg.Objects, "ApexClass")
	payload := struct {
		Org       storage.OrgState `json:"org"`
		PageNames []string         `json:"pageNames,omitempty"`
	}{
		Org:       ambientOrg,
		PageNames: append([]string(nil), pageNames...),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), true
}

func runtimePatchRuntimeInputsFingerprint(org storage.OrgState, pageNames []string, methods map[string]vm.Method) (string, bool) {
	ambientFingerprint, ok := runtimePatchAmbientFingerprint(org, pageNames)
	if !ok {
		return "", false
	}
	apiVersionFingerprint, ok := runtimePatchCompiledAPIVersionFingerprint(methods)
	if !ok {
		return "", false
	}
	return runtimePatchRuntimeInputsFingerprintFromParts(ambientFingerprint, apiVersionFingerprint), true
}

func runtimePatchRuntimeInputsFingerprintFromParts(ambientFingerprint, apiVersionFingerprint string) string {
	digest := sha256.Sum256([]byte(ambientFingerprint + "\x00" + apiVersionFingerprint))
	return hex.EncodeToString(digest[:])
}

func runtimePatchCompiledAPIVersionFingerprint(methods map[string]vm.Method) (string, bool) {
	keys := make([]string, 0, len(methods))
	for key := range methods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, key := range keys {
		method := methods[key]
		for _, value := range []string{key, method.ClassName, method.File, method.APIVersion} {
			_, _ = h.Write([]byte(value))
			_, _ = h.Write([]byte{0})
		}
		if method.Dependency {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func runtimePatchLiveAPIVersionFingerprint(methods map[string]vm.Method, sources *sourceCache) (string, bool) {
	live := make(map[string]vm.Method, len(methods))
	for key, method := range methods {
		if method.File != "" {
			method.APIVersion = sources.apexAPIVersion(method.File)
		}
		live[key] = method
	}
	return runtimePatchCompiledAPIVersionFingerprint(live)
}

func runtimePatchAppliedOutcome(index typesys.Index, affected []runtimePatchAffectedOwner, key runtimeCacheKey, entry runtimeCacheEntry) runtimePatchOutcome {
	recompiledOwners, recompiledPaths := runtimePatchOwnerFacts(affected)
	reusedOwners, reusedPaths := runtimePatchReusedFacts(index, affected)
	return runtimePatchOutcome{
		Applied:          true,
		RecompiledOwners: recompiledOwners,
		RecompiledPaths:  recompiledPaths,
		ReusedOwners:     reusedOwners,
		ReusedPaths:      reusedPaths,
		Key:              key,
		EntryValid:       entry.restored.Valid(),
		TemplateValid:    entry.restored.Valid(),
	}
}

func runtimePatchFullBuildOutcome(index typesys.Index, key runtimeCacheKey, entry runtimeCacheEntry) runtimePatchOutcome {
	owners := make([]runtimePatchAffectedOwner, 0, len(index.Types)+len(index.Triggers))
	for _, typ := range index.Types {
		if typ.File != "" && !typ.Artifact {
			owners = append(owners, runtimePatchAffectedOwner{Name: typ.Name, Namespace: typ.Namespace, Path: typ.File})
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.File != "" {
			owners = append(owners, runtimePatchAffectedOwner{Name: trigger.Name, Namespace: trigger.Namespace, Path: trigger.File})
		}
	}
	recompiledOwners, recompiledPaths := runtimePatchOwnerFacts(owners)
	return runtimePatchOutcome{RecompiledOwners: recompiledOwners, RecompiledPaths: recompiledPaths, Key: key, EntryValid: entry.restored.Valid(), TemplateValid: entry.restored.Valid()}
}

func runtimePatchOwnerFacts(owners []runtimePatchAffectedOwner) ([]string, []string) {
	ownerSet := make(map[string]bool, len(owners))
	pathSet := make(map[string]bool, len(owners))
	for _, owner := range owners {
		ownerSet[runtimePatchClassKey(owner.Namespace, owner.Name)] = true
		pathSet[owner.Path] = true
	}
	ownerFacts := make([]string, 0, len(ownerSet))
	for owner := range ownerSet {
		ownerFacts = append(ownerFacts, owner)
	}
	pathFacts := make([]string, 0, len(pathSet))
	for path := range pathSet {
		pathFacts = append(pathFacts, path)
	}
	sort.Strings(ownerFacts)
	sort.Strings(pathFacts)
	return ownerFacts, pathFacts
}

func runtimePatchReusedFacts(index typesys.Index, affected []runtimePatchAffectedOwner) ([]string, []string) {
	affectedKeys := make(map[string]bool, len(affected))
	for _, owner := range affected {
		affectedKeys[runtimePatchOwnerKey(owner)] = true
	}
	reused := make([]runtimePatchAffectedOwner, 0, len(index.Types))
	for _, typ := range index.Types {
		owner := runtimePatchAffectedOwner{Name: typ.Name, Namespace: typ.Namespace, Path: typ.File}
		if typ.File != "" && !typ.Artifact && !affectedKeys[runtimePatchOwnerKey(owner)] {
			reused = append(reused, owner)
		}
	}
	return runtimePatchOwnerFacts(reused)
}

func runtimePatchOwnerKey(owner runtimePatchAffectedOwner) string {
	return runtimePatchClassKey(owner.Namespace, owner.Name) + "\x00" + owner.Path
}

func runtimePatchClassKey(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "." + name
}

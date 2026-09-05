package apextest

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const runtimePatchABI = "apextest-runtime-patch-v3"

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

type runtimePatchTransitionFlightIdentity struct {
	PredecessorKey                runtimeCacheKey
	PredecessorFingerprint        string
	PredecessorPayloadFingerprint string
	CurrentKey                    runtimeCacheKey
	CurrentFingerprint            string
	RuntimeInputsFingerprint      string
	AffectedOwners                []runtimePatchAffectedOwner
	ABI                           string
}

func (identity runtimePatchTransitionFlightIdentity) key() (string, bool) {
	if identity.PredecessorKey == "" ||
		identity.PredecessorFingerprint == "" ||
		identity.PredecessorPayloadFingerprint == "" ||
		identity.CurrentKey == "" ||
		identity.CurrentFingerprint == "" ||
		identity.RuntimeInputsFingerprint == "" ||
		len(identity.AffectedOwners) == 0 ||
		identity.ABI == "" {
		return "", false
	}
	owners := make([]string, 0, len(identity.AffectedOwners))
	seen := make(map[string]bool, len(identity.AffectedOwners))
	for _, owner := range identity.AffectedOwners {
		key := runtimePatchOwnerKey(owner)
		if key == "" || seen[key] {
			return "", false
		}
		seen[key] = true
		owners = append(owners, key)
	}
	sort.Strings(owners)
	h := sha256.New()
	write := func(value string) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(value))
	}
	write(string(identity.PredecessorKey))
	write(identity.PredecessorFingerprint)
	write(identity.PredecessorPayloadFingerprint)
	write(string(identity.CurrentKey))
	write(identity.CurrentFingerprint)
	write(identity.RuntimeInputsFingerprint)
	write(identity.ABI)
	for _, owner := range owners {
		write(owner)
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

type runtimePatchTransitionFlightResult struct {
	key            runtimeCacheKey
	entry          runtimeCacheEntry
	outcome        runtimePatchOutcome
	applied        bool
	estimatedBytes uint64
}

type runtimePatchTransitionFlight struct {
	done   chan struct{}
	epoch  uint64
	result runtimePatchTransitionFlightResult
	err    error
}

type runtimePatchTransitionFlightGroup struct {
	mu            sync.Mutex
	publicationMu sync.Mutex
	workMu        sync.Mutex
	workCond      *sync.Cond
	activeWork    int
	flights       map[string]*runtimePatchTransitionFlight
	epoch         uint64
}

var errRuntimePatchTransitionReset = errors.New("runtime patch transition reset during shared work")

func (group *runtimePatchTransitionFlightGroup) do(
	ctx context.Context,
	key string,
	build func(epoch uint64) runtimePatchTransitionFlightResult,
) (runtimePatchTransitionFlightResult, bool, error) {
	if err := ctx.Err(); err != nil {
		return runtimePatchTransitionFlightResult{}, false, err
	}
	group.mu.Lock()
	if group.flights == nil {
		group.flights = make(map[string]*runtimePatchTransitionFlight)
	}
	if existing := group.flights[key]; existing != nil {
		group.mu.Unlock()
		invokeSnapshotValidationHook("runtime_patch_transition_flight_waiter")
		select {
		case <-existing.done:
			return existing.result, true, existing.err
		case <-ctx.Done():
			return runtimePatchTransitionFlightResult{}, true, ctx.Err()
		}
	}
	flight := &runtimePatchTransitionFlight{done: make(chan struct{}), epoch: group.epoch}
	group.flights[key] = flight
	group.beginWork()
	group.mu.Unlock()

	go func() {
		defer group.endWork()
		invokeSnapshotValidationHook("runtime_patch_transition_flight_leader")
		result := build(flight.epoch)

		group.mu.Lock()
		if group.epoch == flight.epoch {
			flight.result = result
		} else {
			flight.err = errRuntimePatchTransitionReset
		}
		if group.flights[key] == flight {
			delete(group.flights, key)
		}
		close(flight.done)
		group.mu.Unlock()
	}()
	select {
	case <-flight.done:
		return flight.result, false, flight.err
	case <-ctx.Done():
		return runtimePatchTransitionFlightResult{}, false, ctx.Err()
	}
}

func (group *runtimePatchTransitionFlightGroup) publish(epoch uint64, publish func() bool) bool {
	group.publicationMu.Lock()
	defer group.publicationMu.Unlock()
	group.mu.Lock()
	current := group.epoch == epoch
	group.mu.Unlock()
	return current && publish()
}

func (group *runtimePatchTransitionFlightGroup) reset(resetPublishedState func()) {
	group.publicationMu.Lock()
	group.mu.Lock()
	group.epoch++
	group.flights = make(map[string]*runtimePatchTransitionFlight)
	group.mu.Unlock()
	if resetPublishedState != nil {
		resetPublishedState()
	}
	group.publicationMu.Unlock()
}

func (group *runtimePatchTransitionFlightGroup) Reset() {
	group.reset(nil)
}

// Wait blocks until all detached transition computations have completed.
// Callers must first stop admitting new work.
func (group *runtimePatchTransitionFlightGroup) Wait() {
	group.workMu.Lock()
	for group.activeWork != 0 {
		group.workCondition().Wait()
	}
	group.workMu.Unlock()
}

func (group *runtimePatchTransitionFlightGroup) beginWork() {
	group.workMu.Lock()
	group.activeWork++
	group.workMu.Unlock()
}

func (group *runtimePatchTransitionFlightGroup) endWork() {
	group.workMu.Lock()
	group.activeWork--
	if group.activeWork == 0 {
		group.workCondition().Broadcast()
	}
	group.workMu.Unlock()
}

func (group *runtimePatchTransitionFlightGroup) workCondition() *sync.Cond {
	if group.workCond == nil {
		group.workCond = sync.NewCond(&group.workMu)
	}
	return group.workCond
}

var runtimePatchTransitionFlights runtimePatchTransitionFlightGroup

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
	return newRuntimePatchAuthorityWithPerf(index, key, digests, sources, entry, org, nil)
}

func newRuntimePatchAuthorityWithPerf(index typesys.Index, key runtimeCacheKey, digests *typesys.SourceDigestSet, sources *sourceCache, entry runtimeCacheEntry, org storage.OrgState, counters *runPerfCounters) *runtimePatchAuthority {
	if digests == nil || !runtimePatchDigestSetsEqual(index, digests.Digest) {
		return nil
	}
	runtimeInputsFingerprint, ok := runtimePatchRuntimeInputsFingerprint(org, entry.PageNames, entry.Methods)
	if !ok {
		return nil
	}
	payloadFingerprint, ok := runtimePatchCompiledPayloadFingerprintWithPerf(entry, counters)
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

func runtimePatchAuthorityFromTransition(index typesys.Index, key, predecessorKey runtimeCacheKey, predecessorFingerprint, runtimeInputsFingerprint string, previous *runtimePatchAuthority, affected []runtimePatchAffectedOwner, sources *sourceCache, entry runtimeCacheEntry, counters *runPerfCounters) *runtimePatchAuthority {
	if previous == nil {
		return nil
	}
	fingerprint, ok := runtimePatchIndexFingerprint(index)
	if !ok {
		return nil
	}
	payloadFingerprint, ok := runtimePatchCompiledPayloadFingerprintWithPerf(entry, counters)
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

func runtimePatchAuthorityMatchesPayload(entry runtimeCacheEntry) bool {
	if entry.patchAuthority == nil || entry.patchAuthority.payloadFingerprint == "" {
		return false
	}
	fingerprint, ok := runtimePatchCompiledPayloadFingerprint(entry)
	return ok && fingerprint == entry.patchAuthority.payloadFingerprint
}

func runtimeFromIndexWithSourceDigestsAfter(previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool) (runtimeCacheKey, runtimeCacheEntry, error) {
	affected, _ := runtimePatchOneModifiedOwner(previous, current)
	key, entry, _, err := runtimeFromIndexTransitionContext(context.Background(), previous, current, digests, sources, diskCacheAllowed, nil, affected)
	return key, entry, err
}

func runtimeFromIndexWithSourceDigestsAfterAndPerf(previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters) (runtimeCacheKey, runtimeCacheEntry, error) {
	planningStarted := time.Now()
	affected, _ := runtimePatchOneModifiedOwner(previous, current)
	counters.phases.runtimeKeyNS.Add(time.Since(planningStarted).Nanoseconds())
	key, entry, _, err := runtimeFromIndexTransitionContext(context.Background(), previous, current, digests, sources, diskCacheAllowed, counters, affected)
	return key, entry, err
}

func runtimeFromIndexTransition(previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters, affected []runtimePatchAffectedOwner) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, error) {
	return runtimeFromIndexTransitionContext(context.Background(), previous, current, digests, sources, diskCacheAllowed, counters, affected)
}

func runtimeFromIndexTransitionContext(ctx context.Context, previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters, affected []runtimePatchAffectedOwner) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", runtimeCacheEntry{}, runtimePatchOutcome{}, err
	}
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
	if sources == nil {
		sources = newSourceCache()
	}
	generation, err := prepareRuntimeGeneration(current, digests, sources)
	if err != nil {
		keyStarted := time.Now()
		key := runtimeKeyWithDigestLookup(current, current.SourceDigest, os.ReadFile)
		if counters != nil {
			counters.phases.runtimeKeyNS.Add(time.Since(keyStarted).Nanoseconds())
		}
		evictSnapshotCaches(key)
		return key, runtimeCacheEntry{}, runtimePatchOutcome{Key: key}, err
	}
	var key runtimeCacheKey
	var entry runtimeCacheEntry
	var outcome runtimePatchOutcome
	var applied bool
	key, entry, outcome, applied, err = tryRuntimePatchTransitionContext(ctx, previous, current, digests, sources, generation, counters, affected)
	if err != nil {
		return key, runtimeCacheEntry{}, runtimePatchOutcome{Key: key}, err
	}
	if applied {
		return key, entry, outcome, nil
	}
	if counters == nil {
		key, entry, err = runtimeFromIndexWithPreparedGenerationProjected(current, digests, sources, &generation, cloneRuntimeCacheEntryChecked, diskCacheAllowed)
	} else {
		key, entry, err = runtimeFromIndexWithPreparedGenerationAndPerfProjected(current, digests, sources, &generation, diskCacheAllowed, counters, cloneRuntimeCacheEntryChecked)
	}
	if err != nil {
		return key, entry, runtimePatchOutcome{Key: key}, err
	}
	outcome = runtimePatchFullBuildOutcome(current, key, entry)
	return key, entry, outcome, err
}

func tryRuntimePatchTransitionContext(ctx context.Context, previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, generation runtimeGeneration, counters *runPerfCounters, affected []runtimePatchAffectedOwner) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, bool, error) {
	started := time.Now()
	currentFingerprint, currentOK := runtimePatchIndexFingerprint(current)
	previousFingerprint, previousOK := runtimePatchIndexFingerprint(previous)
	currentKey := generation.key
	previousBaseKey := runtimeKeyWithDigestLookup(previous, previous.SourceDigest, os.ReadFile)
	previousKey := runtimeKeyWithMetadataGeneration(previousBaseKey, generation.metadata)
	if counters != nil {
		counters.phases.runtimeKeyNS.Add(time.Since(started).Nanoseconds())
	}
	if !currentOK || !previousOK || len(affected) == 0 || !runtimePatchTransitionShapeSafe(previous, current, affected) {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
	}

	ambientStarted := time.Now()
	currentOrg := orgFromIndex(current, sources)
	currentPageNames := visualforcePageNames(current)
	currentAmbientFingerprint, ambientOK := runtimePatchAmbientFingerprint(currentOrg, currentPageNames)
	if counters != nil {
		counters.phases.orgBuildNS.Add(time.Since(ambientStarted).Nanoseconds())
	}
	if !ambientOK || sources.sourceSnapshotError() != nil {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
	}

	if cached, ok := validMemoryRuntimeEntry(currentKey); ok {
		currentAPIVersionFingerprint, apiVersionsOK := runtimePatchLiveAPIVersionFingerprint(cached.Methods, sources)
		currentRuntimeInputsFingerprint := runtimePatchRuntimeInputsFingerprintFromParts(currentAmbientFingerprint, currentAPIVersionFingerprint)
		if apiVersionsOK {
			if outcome, trusted := runtimePatchTrustedCacheOutcome(current, currentKey, currentFingerprint, currentRuntimeInputsFingerprint, previousKey, previousFingerprint, affected, cached); trusted {
				invokeSnapshotValidationHook("runtime_patch_before_memory_cache_return")
				if err := sources.validateCapturedSourceGeneration(); err != nil {
					evictSnapshotCaches(currentKey)
					return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
				}
				if counters != nil {
					counters.phases.memoryCacheHits.Add(1)
				}
				return currentKey, cached, outcome, true, nil
			}
		}
		runtimePatchEvictUntrustedCurrentEntry(current, currentKey, currentFingerprint, currentRuntimeInputsFingerprint, previousKey, previousFingerprint, affected)
	}

	previousEntry, ok := runtimePatchTransitionBaseSnapshot(previousKey, previousFingerprint)
	if !ok {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
	}
	currentAPIVersionFingerprint, apiVersionsOK := runtimePatchLiveAPIVersionFingerprint(previousEntry.Methods, sources)
	if !apiVersionsOK {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
	}
	currentRuntimeInputsFingerprint := runtimePatchRuntimeInputsFingerprintFromParts(currentAmbientFingerprint, currentAPIVersionFingerprint)
	if previousEntry.patchAuthority.runtimeInputsFingerprint != currentRuntimeInputsFingerprint ||
		!runtimePatchAffectedClosureValid(current, affected) {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
	}
	flightKey, ok := (runtimePatchTransitionFlightIdentity{
		PredecessorKey:                previousKey,
		PredecessorFingerprint:        previousFingerprint,
		PredecessorPayloadFingerprint: previousEntry.patchAuthority.payloadFingerprint,
		CurrentKey:                    currentKey,
		CurrentFingerprint:            currentFingerprint,
		RuntimeInputsFingerprint:      currentRuntimeInputsFingerprint,
		AffectedOwners:                affected,
		ABI:                           runtimePatchABI,
	}).key()
	if !ok {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
	}

	result, shared, waitErr := runtimePatchTransitionFlights.do(ctx, flightKey, func(epoch uint64) runtimePatchTransitionFlightResult {
		key, _, outcome, applied := buildRuntimePatchTransition(previous, current, digests, sources, generation, counters, affected, epoch)
		result := runtimePatchTransitionFlightResult{key: key, outcome: outcome, applied: applied}
		if !applied {
			return result
		}
		runtimeCacheMu.RLock()
		published, exists := runtimeCache[key]
		runtimeCacheMu.RUnlock()
		if !exists {
			result.applied = false
			return result
		}
		result.entry = published
		result.estimatedBytes = runtimePatchEstimatedCompiledBytes(published)
		return result
	})
	if counters != nil {
		if shared {
			counters.runtimeTransitionWaiters.Add(1)
		} else {
			counters.runtimeTransitionLeaders.Add(1)
		}
	}
	if waitErr != nil {
		if counters != nil && errors.Is(waitErr, context.Canceled) {
			counters.runtimeTransitionCancellations.Add(1)
		}
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false, waitErr
	}
	if !result.applied {
		if counters != nil && shared {
			counters.runtimeTransitionSharedFallbacks.Add(1)
		}
		return result.key, runtimeCacheEntry{}, result.outcome, false, nil
	}
	privateEntry, cloneOK := cloneRuntimeCacheEntryChecked(result.entry)
	if !cloneOK || !privateEntry.restored.Valid() {
		return result.key, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
	}
	invokeSnapshotValidationHook("runtime_patch_before_memory_cache_return")
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		evictSnapshotCaches(result.key)
		return result.key, runtimeCacheEntry{}, runtimePatchOutcome{}, false, nil
	}
	if counters != nil && shared {
		counters.runtimeTransitionSavedBytes.Add(result.estimatedBytes)
		counters.phases.memoryCacheHits.Add(1)
	}
	return result.key, privateEntry, cloneRuntimePatchOutcome(result.outcome), true, nil
}

func cloneRuntimePatchOutcome(outcome runtimePatchOutcome) runtimePatchOutcome {
	outcome.RecompiledOwners = append([]string(nil), outcome.RecompiledOwners...)
	outcome.RecompiledPaths = append([]string(nil), outcome.RecompiledPaths...)
	outcome.ReusedOwners = append([]string(nil), outcome.ReusedOwners...)
	outcome.ReusedPaths = append([]string(nil), outcome.ReusedPaths...)
	return outcome
}

func runtimePatchTransitionBaseSnapshot(key runtimeCacheKey, fingerprint string) (runtimeCacheEntry, bool) {
	runtimeCacheMu.RLock()
	entry, ok := runtimeCache[key]
	runtimeCacheMu.RUnlock()
	if !ok || !entry.restored.Valid() || entry.patchAuthority == nil ||
		entry.patchAuthority.key != key ||
		entry.patchAuthority.fingerprint != fingerprint ||
		entry.patchAuthority.payloadFingerprint == "" ||
		entry.BaseErr != nil || len(entry.TriggerErrors) != 0 {
		return runtimeCacheEntry{}, false
	}
	return entry, true
}

func runtimePatchEstimatedCompiledBytes(entry runtimeCacheEntry) uint64 {
	estimated := uint64(len(entry.Methods))*192 +
		uint64(len(entry.Classes))*256 +
		uint64(len(entry.Triggers))*192 +
		uint64(len(entry.PageNames))*32
	for name, method := range entry.Methods {
		estimated += uint64(len(name) + len(method.Name) + len(method.ClassName) + len(method.Program.Source))
		estimated += uint64(len(method.Program.Instructions)) * 64
	}
	if estimated == 0 {
		return 1
	}
	return estimated
}

func buildRuntimePatchTransition(previous, current typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, generation runtimeGeneration, counters *runPerfCounters, affected []runtimePatchAffectedOwner, flightEpoch uint64) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, bool) {
	started := time.Now()
	currentFingerprint, currentOK := runtimePatchIndexFingerprint(current)
	previousFingerprint, previousOK := runtimePatchIndexFingerprint(previous)
	currentKey := generation.key
	previousBaseKey := runtimeKeyWithDigestLookup(previous, previous.SourceDigest, os.ReadFile)
	previousKey := runtimeKeyWithMetadataGeneration(previousBaseKey, generation.metadata)
	if counters != nil {
		counters.phases.runtimeKeyNS.Add(time.Since(started).Nanoseconds())
	}
	if !currentOK || !previousOK || len(affected) == 0 || !runtimePatchTransitionShapeSafe(previous, current, affected) {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
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
				invokeSnapshotValidationHook("runtime_patch_before_memory_cache_return")
				if err := sources.validateCapturedSourceGeneration(); err != nil {
					evictSnapshotCaches(currentKey)
					return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
				}
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
	patchAuthority := runtimePatchAuthorityFromTransition(current, currentKey, previousKey, previousFingerprint, currentRuntimeInputsFingerprint, previousEntry.patchAuthority, affected, sources, entry, counters)
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
	entry.executionProjectionValidated = true
	invokeSnapshotValidationHook("runtime_patch_before_memory_cache_publication")
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		evictSnapshotCaches(currentKey)
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}

	var resultEntry runtimeCacheEntry
	var resultOutcome runtimePatchOutcome
	cacheHit := false
	published := runtimePatchTransitionFlights.publish(flightEpoch, func() bool {
		runtimeCacheMu.Lock()
		defer runtimeCacheMu.Unlock()
		base, baseOK := runtimeCache[previousKey]
		baseAuthorityOK := baseOK && runtimePatchBaseEntryTrusted(base, previousKey, previousFingerprint, currentRuntimeInputsFingerprint)
		if !baseAuthorityOK {
			return false
		}
		if existing, exists := runtimeCache[currentKey]; exists && existing.restored.Valid() {
			if outcome, trusted := runtimePatchTrustedCacheOutcome(current, currentKey, currentFingerprint, currentRuntimeInputsFingerprint, previousKey, previousFingerprint, affected, existing); trusted {
				cloned, clonedOK := cloneRuntimeCacheEntryChecked(existing)
				if clonedOK && cloned.restored.Valid() {
					resultEntry = cloned
					resultOutcome = outcome
					cacheHit = true
					return true
				}
			}
			delete(runtimeCache, currentKey)
		}
		runtimeCache[currentKey] = entry
		resultEntry = clonedEntry
		resultOutcome = runtimePatchAppliedOutcome(current, affected, currentKey, clonedEntry)
		return true
	})
	if !published {
		return currentKey, runtimeCacheEntry{}, runtimePatchOutcome{}, false
	}
	if counters != nil {
		if cacheHit {
			counters.phases.memoryCacheHits.Add(1)
		} else {
			counters.phases.cacheMisses.Add(1)
		}
	}
	return currentKey, resultEntry, resultOutcome, true
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
	return ir.Program{Instructions: instructions, Source: in.Source, APIVersion: in.APIVersion, Trigger: in.Trigger}, true
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
	ambientOrg := org
	ambientOrg.Objects = maps.Clone(org.Objects)
	delete(ambientOrg.Objects, "ApexClass")
	// JSON only reads field definitions. Avoid cloning their large maps, while
	// retaining Clone's nil/empty normalization for the rest of the payload.
	// The field slices that Clone normalizes all have omitempty JSON tags.
	for name, object := range ambientOrg.Objects {
		object.Definition.Fields = nil
		ambientOrg.Objects[name] = object
	}
	ambientOrg = ambientOrg.Clone()
	for name, object := range ambientOrg.Objects {
		object.Definition.Fields = org.Objects[name].Definition.Fields
		ambientOrg.Objects[name] = object
	}
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

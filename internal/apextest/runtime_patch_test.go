package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestExplicitRuntimeTransitionPatchesOneModifiedOwnerWithoutMutatingBase(t *testing.T) {
	for _, tc := range []struct {
		name string
		load func(typesys.Index, typesys.Index) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, *runPerfCounters, error)
	}{
		{
			name: "normal",
			load: func(previous, current typesys.Index) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, *runPerfCounters, error) {
				affected, _ := runtimePatchOneModifiedOwner(previous, current)
				key, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
				return key, entry, outcome, nil, err
			},
		},
		{
			name: "perf",
			load: func(previous, current typesys.Index) (runtimeCacheKey, runtimeCacheEntry, runtimePatchOutcome, *runPerfCounters, error) {
				counters := newRunPerfCounters(true)
				affected, _ := runtimePatchOneModifiedOwner(previous, current)
				key, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, counters, affected)
				return key, entry, outcome, counters, err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			InvalidateRuntimeCaches()
			t.Cleanup(InvalidateRuntimeCaches)
			fixture := newRuntimeTransitionFixture(t)
			previous, previousDigests := fixture.fullIndex(t)
			previousKey, previousEntry, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false)
			if err != nil {
				t.Fatal(err)
			}

			writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
			current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
			currentKey, currentEntry, outcome, counters, err := tc.load(previous, current)
			if err != nil {
				t.Fatal(err)
			}
			if currentKey == previousKey {
				t.Fatalf("transition retained previous runtime key %q", currentKey)
			}
			if counters != nil {
				phases := snapshotPerfCounters(counters).Phases
				if phases.CacheMisses != 1 || phases.MemoryCacheHits != 0 || phases.DiskCacheHits != 0 {
					t.Fatalf("transition cache counters = miss:%d memory:%d disk:%d", phases.CacheMisses, phases.MemoryCacheHits, phases.DiskCacheHits)
				}
				if phases.RuntimeKeyNS <= 0 || phases.ProjectCompileNS <= 0 || phases.OrgBuildNS <= 0 {
					t.Fatalf("transition phase counters = %#v", phases)
				}
			}
			if !outcome.Applied {
				t.Fatalf("transition did not apply runtime patch: %#v", outcome)
			}
			if got, want := outcome.RecompiledOwners, []string{"ChangedOwner"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("recompiled owners = %v, want %v", got, want)
			}
			if got, want := outcome.ReusedOwners, []string{"StableOwner"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("reused owners = %v, want %v", got, want)
			}

			previousChanged := transitionMethod(t, previousEntry.Methods, "ChangedOwner", "value")
			currentChanged := transitionMethod(t, currentEntry.Methods, "ChangedOwner", "value")
			previousStable := transitionMethod(t, previousEntry.Methods, "StableOwner", "value")
			currentStable := transitionMethod(t, currentEntry.Methods, "StableOwner", "value")
			if !strings.Contains(previousChanged.Program.Source, "return 1") {
				t.Fatalf("previous source changed: %q", previousChanged.Program.Source)
			}
			if !strings.Contains(currentChanged.Program.Source, "return 2") {
				t.Fatalf("current source stale: %q", currentChanged.Program.Source)
			}
			if reflect.DeepEqual(previousChanged, currentChanged) {
				t.Fatal("modified owner compiled content did not change")
			}
			if !reflect.DeepEqual(previousStable, currentStable) {
				t.Fatal("unchanged digest-matched owner compiled content changed")
			}

			previousClone := previousEntry.restored.CloneMachine(nil)
			currentClone := currentEntry.restored.CloneMachine(nil)
			if previousClone == nil || currentClone == nil {
				t.Fatal("transition did not return two valid immutable templates")
			}
			if got := transitionMethod(t, previousClone.Methods, "ChangedOwner", "value").Program.Source; !strings.Contains(got, "return 1") {
				t.Fatalf("previous template mutated: %q", got)
			}
			if got := transitionMethod(t, currentClone.Methods, "ChangedOwner", "value").Program.Source; !strings.Contains(got, "return 2") {
				t.Fatalf("current template stale: %q", got)
			}

			InvalidateRuntimeCaches()
			_, forcedClean, err := runtimeFromIndexWithSourceDigests(current, nil, newSourceCache(), false)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := runtimeTransitionCompiledJSON(t, currentEntry), runtimeTransitionCompiledJSON(t, forcedClean); got != want {
				t.Fatalf("patched runtime differs from forced clean compiled runtime: %s", runtimeTransitionFirstDifference(got, want))
			}
			if got, want := runtimeTransitionCanonicalJSON(t, currentEntry.restored.CloneOrg()), runtimeTransitionCanonicalJSON(t, forcedClean.restored.CloneOrg()); got != want {
				t.Fatalf("patched org differs from forced clean build: %s", runtimeTransitionFirstDifference(got, want))
			}

			currentEntry.Methods["transition-alias-probe"] = vm.Method{Name: "transition-alias-probe"}
			if _, ok := previousEntry.Methods["transition-alias-probe"]; ok {
				t.Fatal("transition aliased the previous methods map")
			}
		})
	}
}

func TestRuntimeTransitionOutcomeReportsAffectedClosureFacts(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	previousKey, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	runtimeCacheMu.RLock()
	base := runtimeCache[previousKey]
	runtimeCacheMu.RUnlock()
	if _, ok := runtimePatchCloneMethods(base.Methods); !ok {
		t.Fatal("method payload clone failed")
	}
	for _, class := range base.Classes {
		if _, ok := runtimePatchCloneClass(class); !ok {
			t.Fatalf("class payload clone failed for %s", class.Name)
		}
	}
	for _, trigger := range base.Triggers {
		if _, ok := runtimePatchCloneProgram(trigger.Program, make(map[*ir.Expr]bool), 0); !ok {
			t.Fatalf("trigger payload clone failed for %s", trigger.Name)
		}
	}
	cloned, ok := cloneRuntimeCacheEntryChecked(base)
	if !ok {
		t.Fatal("complete payload clone failed")
	}
	if !reflect.DeepEqual(base.Methods, cloned.Methods) {
		t.Fatal("method clone changed payload shape")
	}
	if !reflect.DeepEqual(base.Classes, cloned.Classes) {
		t.Fatal("class clone changed payload shape")
	}
	if !reflect.DeepEqual(base.Triggers, cloned.Triggers) {
		t.Fatal("trigger clone changed payload shape")
	}
	if !reflect.DeepEqual(base.TriggerErrors, cloned.TriggerErrors) || !reflect.DeepEqual(base.PageNames, cloned.PageNames) || runtimePatchErrorIdentity(base.BaseErr) != runtimePatchErrorIdentity(cloned.BaseErr) {
		t.Fatal("error or page clone changed payload shape")
	}
	baseValues, baseValuesOK := runtimePatchClassValueFingerprints(base.Classes)
	clonedValues, clonedValuesOK := runtimePatchClassValueFingerprints(cloned.Classes)
	if !baseValuesOK || !clonedValuesOK || !reflect.DeepEqual(baseValues, clonedValues) {
		t.Fatal("class value supplement changed payload shape")
	}
	if !runtimePatchAuthorityMatchesPayload(cloned) {
		baseFingerprint, _ := runtimePatchCompiledPayloadFingerprint(base)
		clonedFingerprint, _ := runtimePatchCompiledPayloadFingerprint(cloned)
		t.Fatalf("complete clone changed payload authority: base %s clone %s stored %s", baseFingerprint, clonedFingerprint, base.patchAuthority.payloadFingerprint)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe transition did not produce an affected-owner closure")
	}
	key, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Applied || outcome.Key != key || !outcome.EntryValid || !outcome.TemplateValid || !entry.restored.Valid() {
		t.Fatalf("transition outcome = %#v", outcome)
	}
	if got, want := outcome.RecompiledOwners, []string{"ChangedOwner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("recompiled owners = %v, want %v", got, want)
	}
	if got, want := outcome.RecompiledPaths, []string{fixture.changed}; !reflect.DeepEqual(got, want) {
		t.Fatalf("recompiled paths = %v, want %v", got, want)
	}
	if got, want := outcome.ReusedOwners, []string{"StableOwner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reused owners = %v, want %v", got, want)
	}
	if got, want := outcome.ReusedPaths, []string{fixture.stable}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reused paths = %v, want %v", got, want)
	}
}

func TestRuntimeTransitionRejectsAffectedClosureThatOmitsChangedPath(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	writeFile(t, fixture.stable, `public class StableOwner { public static Integer value() { return 8; } }`)
	current, _ := fixture.fullIndex(t)
	incomplete := []runtimePatchAffectedOwner{{Name: "ChangedOwner", Path: fixture.changed}}
	_, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, incomplete)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied {
		t.Fatalf("incomplete affected closure applied: %#v", outcome)
	}
	if got := transitionMethod(t, entry.Methods, "StableOwner", "value").Program.Source; !strings.Contains(got, "return 8") {
		t.Fatalf("fallback retained omitted owner's stale source: %q", got)
	}
}

func TestRuntimeTransitionRejectsUnboundCurrentKeyCacheEntry(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	_, previousEntry, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe transition did not produce affected closure")
	}
	currentKey := runtimeKeyWithDigestLookup(current, current.SourceDigest, os.ReadFile)
	poisoned := previousEntry
	poisoned.patchAuthority = nil
	runtimeCacheMu.Lock()
	runtimeCache[currentKey] = poisoned
	runtimeCacheMu.Unlock()

	_, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Applied {
		t.Fatalf("transition did not replace unbound current-key entry: %#v", outcome)
	}
	if got := transitionMethod(t, entry.Methods, "ChangedOwner", "value").Program.Source; !strings.Contains(got, "return 2") {
		t.Fatalf("transition returned unbound current-key runtime: %q", got)
	}
	currentCached, ok := validMemoryRuntimeEntry(currentKey)
	if !ok || currentCached.patchAuthority == nil || currentCached.patchAuthority.key != currentKey {
		t.Fatal("transition did not publish current immutable authority")
	}
}

func TestRuntimeTransitionReportsFullBuildProvenanceForTrustedCurrentHit(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current, currentDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(current, currentDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe transition did not produce affected closure")
	}
	_, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied || len(outcome.ReusedOwners) != 0 || len(outcome.ReusedPaths) != 0 {
		t.Fatalf("trusted full-build hit claimed patch provenance: %#v", outcome)
	}
	if got := transitionMethod(t, entry.Methods, "ChangedOwner", "value").Program.Source; !strings.Contains(got, "return 2") {
		t.Fatalf("trusted current hit source stale: %q", got)
	}
}

func TestRuntimeTransitionFailsClosedWhenLiveOrgOrPageInputsDrift(t *testing.T) {
	for _, warmCurrent := range []bool{false, true} {
		name := "patch_candidate"
		if warmCurrent {
			name = "trusted_current_key_candidate"
		}
		t.Run(name, func(t *testing.T) {
			InvalidateRuntimeCaches()
			t.Cleanup(InvalidateRuntimeCaches)
			fixture := newRuntimeTransitionFixture(t)
			pageBefore := filepath.Join(fixture.root, "force-app/main/default/pages/Before.page")
			application := filepath.Join(fixture.root, "force-app/main/default/applications/Transition.app-meta.xml")
			writeFile(t, pageBefore, `<apex:page/>`)
			writeFile(t, application, `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata"><label>Before</label></CustomApplication>`)

			previous, previousDigests := fixture.fullIndex(t)
			if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
				t.Fatal(err)
			}
			writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
			current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
			if warmCurrent {
				if _, _, err := runtimeFromIndexWithSourceDigests(current, nil, newSourceCache(), false); err != nil {
					t.Fatal(err)
				}
			}

			writeFile(t, filepath.Join(fixture.root, "force-app/main/default/pages/After.page"), `<apex:page/>`)
			writeFile(t, application, `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata"><label>After</label></CustomApplication>`)
			affected, ok := runtimePatchOneModifiedOwner(previous, current)
			if !ok {
				t.Fatal("safe Apex transition did not produce affected closure")
			}
			_, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Applied || len(outcome.ReusedOwners) != 0 || len(outcome.ReusedPaths) != 0 {
				t.Fatalf("runtime transition patched across unbound org/page drift: %#v", outcome)
			}
			if !runtimeTransitionContainsFold(entry.PageNames, "After") {
				t.Fatalf("full-build page authority is stale: %v", entry.PageNames)
			}

			InvalidateRuntimeCaches()
			_, forcedClean, err := runtimeFromIndexWithSourceDigests(current, nil, newSourceCache(), false)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := runtimeTransitionCanonicalJSON(t, entry.restored.CloneOrg()), runtimeTransitionCanonicalJSON(t, forcedClean.restored.CloneOrg()); got != want {
				t.Fatalf("fallback org differs from forced clean after metadata drift: %s", runtimeTransitionFirstDifference(got, want))
			}
			if !reflect.DeepEqual(entry.PageNames, forcedClean.PageNames) {
				t.Fatalf("fallback pages = %v, forced clean = %v", entry.PageNames, forcedClean.PageNames)
			}
		})
	}
}

func TestRuntimeTransitionFailsClosedWhenUnchangedOwnerApexMetadataDrifts(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	stableMetadata := fixture.stable + "-meta.xml"
	writeFile(t, stableMetadata, `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>61.0</apiVersion></ApexClass>`)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}

	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	writeFile(t, stableMetadata, `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion></ApexClass>`)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe Apex transition did not produce affected closure")
	}
	_, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied {
		t.Fatalf("runtime transition patched across unchanged-owner Apex metadata drift: %#v", outcome)
	}
	if got := transitionMethod(t, entry.Methods, "StableOwner", "value").APIVersion; got != "62.0" {
		t.Fatalf("full-build stable owner API version = %q, want 62.0", got)
	}
}

func TestRuntimePatchCompilationUsesOneCapturedEffectiveAPIVersion(t *testing.T) {
	fixture := newRuntimeTransitionFixture(t)
	metadata := fixture.changed + "-meta.xml"
	writeFile(t, metadata, `<ApexClass><apiVersion>61.0</apiVersion></ApexClass>`)
	index, _ := fixture.fullIndex(t)
	sources := newSourceCache()
	if got := sources.apexAPIVersion(fixture.changed); got != "61.0" {
		t.Fatalf("captured API version = %q, want 61.0", got)
	}
	writeFile(t, metadata, `<ApexClass><apiVersion>62.0</apiVersion></ApexClass>`)
	include := func(typ typesys.TypeSymbol) bool { return typ.File == fixture.changed }
	methods := compileProjectMethodsWhere(index, include, sources)
	if got := transitionMethod(t, methods, "ChangedOwner", "value").APIVersion; got != "61.0" {
		t.Fatalf("compiled API version reread sidecar: got %q, want captured 61.0", got)
	}
}

func TestRuntimeFullBuildRefreshesEffectiveAPIVersionOnSameSourceCacheBinding(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	metadata := fixture.changed + "-meta.xml"
	writeFile(t, metadata, `<ApexClass><apiVersion>61.0</apiVersion></ApexClass>`)
	index, digests := fixture.fullIndex(t)
	sources := newSourceCache()
	_, first, err := runtimeFromIndexWithSourceDigests(index, digests, sources, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := transitionMethod(t, first.Methods, "ChangedOwner", "value").APIVersion; got != "61.0" {
		t.Fatalf("first API version = %q, want 61.0", got)
	}
	writeFile(t, metadata, `<ApexClass><apiVersion>62.0</apiVersion></ApexClass>`)
	InvalidateRuntimeCaches()
	_, second, err := runtimeFromIndexWithSourceDigests(index, digests, sources, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := transitionMethod(t, second.Methods, "ChangedOwner", "value").APIVersion; got != "62.0" {
		t.Fatalf("same-cache rebuilt API version = %q, want 62.0", got)
	}
}

func TestRuntimePatchSemanticAuthorityRejectsCallExpressions(t *testing.T) {
	methods := map[string]vm.Method{
		"changedowner.value": {
			Name:      "ChangedOwner.value",
			ClassName: "ChangedOwner",
			Program: ir.Program{Instructions: []ir.Instruction{{
				Op: ir.OpReturn,
				Expr: ir.Expr{
					Kind:   ir.ExprCall,
					Callee: "Schema.describeSObjects",
					Args:   []ir.Expr{{Kind: ir.ExprLiteral, Value: "0"}},
				},
			}}},
		},
	}
	if runtimePatchCompiledMethodsReferenceSafe(methods) {
		t.Fatal("semantic authority accepted a dynamic call expression")
	}
}

func TestRuntimePatchLiveAPIVersionAuthorityIncludesDependencyMethods(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "DependencyOwner.cls")
	writeFile(t, file+"-meta.xml", `<ApexClass><apiVersion>62.0</apiVersion></ApexClass>`)
	methods := map[string]vm.Method{
		"dependencyowner.value": {
			Name:       "DependencyOwner.value",
			ClassName:  "DependencyOwner",
			File:       file,
			APIVersion: "61.0",
			Dependency: true,
		},
	}
	compiled, ok := runtimePatchCompiledAPIVersionFingerprint(methods)
	if !ok {
		t.Fatal("compiled dependency API authority failed")
	}
	live, ok := runtimePatchLiveAPIVersionFingerprint(methods, newSourceCache())
	if !ok {
		t.Fatal("live dependency API authority failed")
	}
	if live == compiled {
		t.Fatal("dependency sidecar drift was omitted from live API authority")
	}
}

func TestRuntimeTransitionCurrentHitMustMatchRequestedPredecessorClosure(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)

	writeFile(t, fixture.stable, `public class StableOwner { public static Integer value() { return 8; } }`)
	previousChanged, previousChangedDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previousChanged, previousChangedDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}

	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	writeFile(t, fixture.stable, `public class StableOwner { public static Integer value() { return 7; } }`)
	previousStable, previousStableDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previousStable, previousStableDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}

	writeFile(t, fixture.stable, `public class StableOwner { public static Integer value() { return 8; } }`)
	currentFromChanged := fixture.incrementalIndex(t, previousChanged, []string{fixture.changed}, nil)
	currentFromStable := fixture.incrementalIndex(t, previousStable, []string{fixture.stable}, nil)
	changedAffected, ok := runtimePatchOneModifiedOwner(previousChanged, currentFromChanged)
	if !ok {
		t.Fatal("changed-owner transition did not produce affected closure")
	}
	stableAffected, ok := runtimePatchOneModifiedOwner(previousStable, currentFromStable)
	if !ok {
		t.Fatal("stable-owner transition did not produce affected closure")
	}
	firstKey, _, firstOutcome, err := runtimeFromIndexTransition(previousChanged, currentFromChanged, nil, newSourceCache(), false, nil, changedAffected)
	if err != nil || !firstOutcome.Applied {
		t.Fatalf("first transition = %#v, err %v", firstOutcome, err)
	}
	secondKey, secondEntry, secondOutcome, err := runtimeFromIndexTransition(previousStable, currentFromStable, nil, newSourceCache(), false, nil, stableAffected)
	if err != nil {
		t.Fatal(err)
	}
	if firstKey != secondKey {
		t.Fatalf("equivalent current snapshots produced different keys: %q != %q", firstKey, secondKey)
	}
	if !secondOutcome.Applied {
		t.Fatalf("second transition did not apply against its requested predecessor: %#v", secondOutcome)
	}
	if got, want := secondOutcome.RecompiledOwners, []string{"StableOwner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second transition reused cached closure facts: got %v, want %v", got, want)
	}
	if got, want := secondOutcome.ReusedOwners, []string{"ChangedOwner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second transition reused owners = %v, want %v", got, want)
	}
	if got := transitionMethod(t, secondEntry.Methods, "StableOwner", "value").Program.Source; !strings.Contains(got, "return 8") {
		t.Fatalf("second transition returned stale stable owner: %q", got)
	}
}

func TestRuntimeTransitionRejectsObfuscatedDynamicTargetChange(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { Type t = Type./* old */forName('StableOwner'); return t == null ? 0 : 1; } }`)
	previous, previousDigests := fixture.fullIndex(t)
	_, previousEntry, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { Type t = Type./* new */forName('ChangedOwner'); return t == null ? 0 : 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected := []runtimePatchAffectedOwner{{Name: "ChangedOwner", Path: fixture.changed}}
	_, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied {
		t.Fatalf("obfuscated dynamic target transition applied: %#v", outcome)
	}
	runtimeTransitionRequireNoReusedSourceInstructions(t, previousEntry, entry)
}

func TestRuntimePatchReferenceFingerprintIncludesLowercaseIdentifiers(t *testing.T) {
	accountReference := runtimePatchStaticReferenceFingerprint(`public static Integer value() { account row; return row == null ? 1 : 0; }`)
	contactReference := runtimePatchStaticReferenceFingerprint(`public static Integer value() { contact row; return row == null ? 2 : 0; }`)
	if accountReference == contactReference {
		t.Fatal("case-insensitive lowercase type reference drift was omitted from the authority fingerprint")
	}
}

func TestRuntimeTransitionFullBuildOutcomeIncludesTriggerPaths(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.trigger, `trigger TransitionTrigger on Account (before insert) { Trigger.new[0].Name = 'changed'; }`)
	current, _ := fixture.fullIndex(t)
	_, entry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied || !entry.restored.Valid() {
		t.Fatalf("trigger change did not use valid full build: %#v", outcome)
	}
	if !runtimeTransitionContainsExact(outcome.RecompiledPaths, fixture.trigger) {
		t.Fatalf("full-build recompiled paths omit trigger %q: %v", fixture.trigger, outcome.RecompiledPaths)
	}
}

func TestRuntimePatchAuthorityUsesRetainedVerifiedSources(t *testing.T) {
	fixture := newRuntimeTransitionFixture(t)
	index, digests := fixture.fullIndex(t)
	sources := newSourceCache()
	sources.configureNamespaceRemaps(index.Types, index.Triggers)
	sources.bindSourceDigests(digests)
	if err := preloadRuntimeSources(index, sources); err != nil {
		t.Fatal(err)
	}
	org := orgFromIndex(index, sources)
	pageNames := visualforcePageNames(index)
	for _, path := range []string{fixture.changed, fixture.stable, fixture.trigger} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	key := runtimeKeyWithSourceDigests(index, digests, os.ReadFile)
	methods := compileProjectMethods(index, sources)
	classes := compileProjectClasses(index, methods, sources)
	triggers, triggerErrors := compileProjectTriggers(index, sources)
	entry := runtimeCacheEntry{Methods: methods, Classes: classes, Triggers: triggers, TriggerErrors: triggerErrors, PageNames: pageNames}
	if authority := newRuntimePatchAuthority(index, key, digests, sources, entry, org); authority == nil {
		t.Fatal("authority reread files instead of using retained verified sources")
	}
}

func TestRuntimeTransitionErrorOutcomeDoesNotClaimSuccessfulRecompile(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe transition did not produce affected closure")
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 3; } }`)
	InvalidateRuntimeCaches()
	_, _, outcome, err := runtimeFromIndexTransition(previous, current, previousDigests, newSourceCache(), false, nil, affected)
	if err == nil {
		t.Fatal("stale digest fallback returned no snapshot error")
	}
	if len(outcome.RecompiledOwners) != 0 || len(outcome.RecompiledPaths) != 0 || outcome.EntryValid || outcome.TemplateValid {
		t.Fatalf("failed fallback claimed successful structural facts: %#v", outcome)
	}
}

func TestRuntimeTransitionRejectsStaleSuppliedDigestAuthorityBeforePatch(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe transition did not produce affected closure")
	}
	_, _, outcome, err := runtimeFromIndexTransition(previous, current, previousDigests, newSourceCache(), false, nil, affected)
	if err == nil {
		t.Fatal("transition accepted stale supplied digest authority")
	}
	if outcome.Applied || len(outcome.RecompiledOwners) != 0 || len(outcome.RecompiledPaths) != 0 || outcome.EntryValid || outcome.TemplateValid {
		t.Fatalf("stale supplied digest failure claimed successful structural facts: %#v", outcome)
	}
}

func TestRuntimeTransitionRejectsCachedBaseFromDifferentPrivateProjectIdentity(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previousCached, previousCachedDigests := fixture.fullIndex(t)
	previousCachedKey, _, err := runtimeFromIndexWithSourceDigests(previousCached, previousCachedDigests, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(fixture.root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":false}]}`)
	previousRequest, _ := fixture.fullIndex(t)
	if typesys.SameProjectIdentity(previousCached, previousRequest) {
		t.Fatal("fixture did not create distinct private project identities")
	}
	previousRequestKey := runtimeKeyWithDigestLookup(previousRequest, previousRequest.SourceDigest, os.ReadFile)
	if previousCachedKey != previousRequestKey {
		t.Fatalf("fixture did not produce a colliding exported runtime key: %q != %q", previousCachedKey, previousRequestKey)
	}

	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previousRequest, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previousRequest, current)
	if !ok {
		t.Fatal("safe request transition did not produce affected closure")
	}
	_, entry, outcome, err := runtimeFromIndexTransition(previousRequest, current, nil, newSourceCache(), false, nil, affected)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied {
		t.Fatalf("transition trusted cached base from a different private identity: %#v", outcome)
	}
	if got := transitionMethod(t, entry.Methods, "ChangedOwner", "value").Program.Source; !strings.Contains(got, "return 2") {
		t.Fatalf("full-build fallback returned stale changed owner: %q", got)
	}
}

func TestExplicitRuntimeTransitionFallsBackForEveryUnsafeShape(t *testing.T) {
	tests := []struct {
		name               string
		mutate             func(*testing.T, runtimeTransitionFixture, typesys.Index) typesys.Index
		breakBaseAuthority bool
		clearBaseCache     bool
		wantTriggerError   bool
		wantBaseError      bool
	}{
		{
			name: "missing current index authority",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
				data, err := json.Marshal(current)
				if err != nil {
					t.Fatal(err)
				}
				var decoded typesys.Index
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatal(err)
				}
				return decoded
			},
		},
		{
			name: "missing previous memory base",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				return fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
			},
			clearBaseCache: true,
		},
		{
			name: "memory base without source authority",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				return fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
			},
			breakBaseAuthority: true,
		},
		{
			name: "multiple modified owners",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				writeFile(t, fixture.stable, `public class StableOwner { public static Integer value() { return 8; } }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "added owner",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, filepath.Join(filepath.Dir(fixture.changed), "AddedOwner.cls"), `public class AddedOwner { public static Integer value() { return 9; } }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "deleted owner",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				if err := os.Remove(fixture.changed); err != nil {
					t.Fatal(err)
				}
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "renamed owner",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				renamed := filepath.Join(filepath.Dir(fixture.changed), "RenamedOwner.cls")
				if err := os.Rename(fixture.changed, renamed); err != nil {
					t.Fatal(err)
				}
				writeFile(t, renamed, `public class RenamedOwner { public static Integer value() { return 2; } }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "trigger modification",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, fixture.trigger, `trigger TransitionTrigger on Account (before insert) { Trigger.new[0].Name = 'changed'; }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name:             "trigger compiler error",
			wantTriggerError: true,
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, fixture.trigger, `trigger TransitionTrigger on Account (before insert) { System.debug('changed'); }`)
				current, _ := fixture.fullIndex(t)
				if len(current.Triggers) != 1 {
					t.Fatalf("trigger count = %d", len(current.Triggers))
				}
				current.Triggers[0].Range.End.Offset = current.Triggers[0].Range.Start.Offset
				return current
			},
		},
		{
			name:          "base registration error",
			wantBaseError: true,
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
				current.Types = append([]typesys.TypeSymbol(nil), current.Types...)
				for i := range current.Types {
					if current.Types[i].File == fixture.changed {
						current.Types[i].Name = ""
						return current
					}
				}
				t.Fatal("changed owner not found")
				return typesys.Index{}
			},
		},
		{
			name: "owner structure drift",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } public static void added() {} }`)
				return fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
			},
		},
		{
			name: "namespace remap drift",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
				current.Types = append([]typesys.TypeSymbol(nil), current.Types...)
				for i := range current.Types {
					if current.Types[i].File == fixture.changed {
						current.Types[i].SourceNamespaceRemaps = []namespaceremap.Rule{{From: "base", To: "stage"}}
					}
				}
				return current
			},
		},
		{
			name: "dependency owner",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
				current.Types = append([]typesys.TypeSymbol(nil), current.Types...)
				for i := range current.Types {
					if current.Types[i].File == fixture.changed {
						current.Types[i].Dependency = true
					}
				}
				return current
			},
		},
		{
			name: "schema drift",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
				current.Objects = append(current.Objects, gladeschema.Object{Name: "Transition__c"})
				return current
			},
		},
		{
			name: "project drift",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
				current.Project.Namespace = "changed"
				return current
			},
		},
		{
			name: "private project identity drift",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, filepath.Join(fixture.root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":false}]}`)
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "static reference drift",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return StableOwner.value(); } }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "dynamic reference",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { Type t = Type.forName('StableOwner'); return t == null ? 0 : 2; } }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "comment obfuscated dynamic type reference",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { Type t = Type./* hidden */forName('StableOwner'); return t == null ? 0 : 2; } }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "dynamic database query",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, _ typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { List<SObject> rows = Database.query('SELECT Id FROM Account'); return rows.size(); } }`)
				current, _ := fixture.fullIndex(t)
				return current
			},
		},
		{
			name: "source moved beyond current snapshot",
			mutate: func(t *testing.T, fixture runtimeTransitionFixture, previous typesys.Index) typesys.Index {
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
				current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
				writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 3; } }`)
				return current
			},
		},
	}

	for _, perf := range []bool{false, true} {
		pathName := "normal"
		if perf {
			pathName = "perf"
		}
		t.Run(pathName, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					InvalidateRuntimeCaches()
					t.Cleanup(InvalidateRuntimeCaches)
					fixture := newRuntimeTransitionFixture(t)
					previous, previousDigests := fixture.fullIndex(t)
					previousKey, previousEntry, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false)
					if err != nil {
						t.Fatal(err)
					}
					current := test.mutate(t, fixture, previous)
					if test.clearBaseCache {
						InvalidateRuntimeCaches()
					}
					if test.breakBaseAuthority {
						runtimeCacheMu.Lock()
						entry := runtimeCache[previousKey]
						entry.patchAuthority = nil
						runtimeCache[previousKey] = entry
						runtimeCacheMu.Unlock()
					}
					affected, _ := runtimePatchOneModifiedOwner(previous, current)
					var counters *runPerfCounters
					if perf {
						counters = newRunPerfCounters(true)
					}
					_, currentEntry, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, counters, affected)
					if err != nil {
						t.Fatal(err)
					}
					if outcome.Applied {
						t.Fatalf("unsafe transition reported applied: %#v", outcome)
					}
					if !currentEntry.restored.Valid() || !outcome.EntryValid || !outcome.TemplateValid {
						t.Fatalf("fallback outcome invalid: %#v", outcome)
					}
					if test.wantTriggerError != (len(currentEntry.TriggerErrors) > 0) {
						t.Fatalf("trigger errors = %v, want error:%t", currentEntry.TriggerErrors, test.wantTriggerError)
					}
					if test.wantBaseError != (currentEntry.BaseErr != nil) {
						t.Fatalf("base error = %v, want error:%t", currentEntry.BaseErr, test.wantBaseError)
					}
					if counters != nil {
						phases := snapshotPerfCounters(counters).Phases
						if phases.CacheMisses != 1 || phases.MemoryCacheHits != 0 || phases.DiskCacheHits != 0 {
							t.Fatalf("fallback counters = miss:%d memory:%d disk:%d", phases.CacheMisses, phases.MemoryCacheHits, phases.DiskCacheHits)
						}
					}
					runtimeTransitionRequireNoReusedSourceInstructions(t, previousEntry, currentEntry)

					InvalidateRuntimeCaches()
					_, forcedClean, err := runtimeFromIndexWithSourceDigests(current, nil, newSourceCache(), false)
					if err != nil {
						t.Fatal(err)
					}
					if got, want := runtimeTransitionCompiledJSON(t, currentEntry), runtimeTransitionCompiledJSON(t, forcedClean); got != want {
						t.Fatal("fallback compiled runtime differs from forced clean build")
					}
					if got, want := runtimeTransitionErrorIdentities(currentEntry.TriggerErrors), runtimeTransitionErrorIdentities(forcedClean.TriggerErrors); !reflect.DeepEqual(got, want) {
						t.Fatalf("fallback trigger errors = %v, forced clean = %v", got, want)
					}
					if got, want := runtimeTransitionErrorIdentity(currentEntry.BaseErr), runtimeTransitionErrorIdentity(forcedClean.BaseErr); got != want {
						t.Fatalf("fallback base error = %q, forced clean = %q", got, want)
					}
					if got, want := runtimeTransitionCanonicalJSON(t, currentEntry.restored.CloneOrg()), runtimeTransitionCanonicalJSON(t, forcedClean.restored.CloneOrg()); got != want {
						t.Fatalf("fallback org differs from forced clean build: %s", runtimeTransitionFirstDifference(got, want))
					}
				})
			}
		})
	}
}

func TestOrdinaryRuntimeEntrypointsDoNotInferPreviousSnapshot(t *testing.T) {
	for _, perf := range []bool{false, true} {
		name := "normal"
		if perf {
			name = "perf"
		}
		t.Run(name, func(t *testing.T) {
			InvalidateRuntimeCaches()
			t.Cleanup(InvalidateRuntimeCaches)
			fixture := newRuntimeTransitionFixture(t)
			previous, previousDigests := fixture.fullIndex(t)
			_, previousEntry, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
			current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
			var currentEntry runtimeCacheEntry
			if perf {
				_, currentEntry, err = runtimeFromIndexWithSourceDigestsAndPerf(current, nil, newSourceCache(), false, newRunPerfCounters(true))
			} else {
				_, currentEntry, err = runtimeFromIndexWithSourceDigests(current, nil, newSourceCache(), false)
			}
			if err != nil {
				t.Fatal(err)
			}
			runtimeTransitionRequireNoReusedSourceInstructions(t, previousEntry, currentEntry)
		})
	}
}

func TestRuntimeTransitionConcurrentPublicationKeepsOneAuthoritativeResult(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	previousKey, previousEntry, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe transition did not produce affected closure")
	}
	const workers = 8
	entries := make([]runtimeCacheEntry, workers)
	keys := make([]runtimeCacheKey, workers)
	outcomes := make([]runtimePatchOutcome, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			keys[i], entries[i], outcomes[i], errs[i] = runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
		}(i)
	}
	close(start)
	wg.Wait()
	for i := range entries {
		if errs[i] != nil || !outcomes[i].Applied || !entries[i].restored.Valid() {
			t.Fatalf("worker %d = outcome:%#v err:%v", i, outcomes[i], errs[i])
		}
		if keys[i] != keys[0] {
			t.Fatalf("worker keys differ: %q != %q", keys[i], keys[0])
		}
		if got := transitionMethod(t, entries[i].Methods, "ChangedOwner", "value").Program.Source; !strings.Contains(got, "return 2") {
			t.Fatalf("worker %d source stale: %q", i, got)
		}
	}
	currentCached, ok := validMemoryRuntimeEntry(keys[0])
	if !ok || currentCached.patchAuthority == nil || currentCached.patchAuthority.key != keys[0] {
		t.Fatal("concurrent transition did not leave one authoritative current cache entry")
	}
	previousCached, ok := validMemoryRuntimeEntry(previousKey)
	if !ok || previousCached.patchAuthority == nil {
		t.Fatal("concurrent transition displaced previous cache authority")
	}
	if got := transitionMethod(t, previousEntry.Methods, "ChangedOwner", "value").Program.Source; !strings.Contains(got, "return 1") {
		t.Fatalf("concurrent transition mutated prior entry: %q", got)
	}
}

func TestRuntimeTransitionRejectsDuplicateRequestedClosureOnCurrentHit(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	complete := []runtimePatchAffectedOwner{
		{Name: "ChangedOwner", Path: fixture.changed},
		{Name: "StableOwner", Path: fixture.stable},
	}
	_, _, first, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, complete)
	if err != nil || !first.Applied {
		t.Fatalf("complete closure transition = %#v, err %v", first, err)
	}
	duplicate := []runtimePatchAffectedOwner{
		{Name: "ChangedOwner", Path: fixture.changed},
		{Name: "ChangedOwner", Path: fixture.changed},
	}
	_, _, second, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if second.Applied {
		t.Fatalf("duplicate requested closure trusted current hit: %#v", second)
	}
}

func TestRuntimeTransitionRevalidatesCachePayloadAfterReturnedEntryMutation(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	writeFile(t, filepath.Join(fixture.root, "force-app/main/default/pages/CacheAuthority.page"), `<apex:page/>`)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe transition did not produce affected closure")
	}
	currentKey, returned, first, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil || !first.Applied {
		t.Fatalf("first transition = %#v, err %v", first, err)
	}
	methodKey := ""
	for key, method := range returned.Methods {
		if method.ClassName == "ChangedOwner" {
			methodKey = key
			method.Program.Source = "poisoned"
			returned.Methods[key] = method
			break
		}
	}
	if methodKey == "" {
		t.Fatal("changed method not found")
	}
	classMethodMutated := false
	for classIndex := range returned.Classes {
		for key, method := range returned.Classes[classIndex].Methods {
			method.Program.Source = "poisoned-class-program"
			returned.Classes[classIndex].Methods[key] = method
			classMethodMutated = true
			break
		}
		if classMethodMutated {
			break
		}
	}
	if !classMethodMutated || len(returned.Triggers) == 0 || len(returned.PageNames) == 0 {
		t.Fatal("fixture lacks nested class, trigger, or page cache payload")
	}
	returned.Triggers[0].Program.Source = "poisoned-trigger-program"
	returned.PageNames[0] = "PoisonedPage"
	returned.patchAuthority.sourceReferences[fixture.changed] = "poisoned-authority"
	_, ordinaryRepaired, err := runtimeFromIndexWithSourceDigests(current, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got := transitionMethod(t, ordinaryRepaired.Methods, "ChangedOwner", "value").Program.Source; got == "poisoned" || !strings.Contains(got, "return 2") {
		t.Fatalf("ordinary cache retrieval trusted poisoned method payload: %q", got)
	}
	if runtimeTransitionContainsFold(ordinaryRepaired.PageNames, "PoisonedPage") || !runtimeTransitionContainsFold(ordinaryRepaired.PageNames, "CacheAuthority") {
		t.Fatalf("ordinary cache retrieval trusted poisoned page payload: %v", ordinaryRepaired.PageNames)
	}
	if ordinaryRepaired.Triggers[0].Program.Source == "poisoned-trigger-program" {
		t.Fatal("ordinary cache retrieval trusted poisoned trigger payload")
	}
	_, repaired, second, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil {
		t.Fatal(err)
	}
	if got := transitionMethod(t, repaired.Methods, "ChangedOwner", "value").Program.Source; got == "poisoned" || !strings.Contains(got, "return 2") {
		t.Fatalf("later transition trusted poisoned cache payload: %q, outcome %#v", got, second)
	}
	cached, ok := validMemoryRuntimeEntry(currentKey)
	if !ok || cached.patchAuthority.sourceReferences[fixture.changed] == "poisoned-authority" {
		t.Fatal("returned authority mutation reached cache-owned authority")
	}
}

func TestRuntimeCacheReturnedPayloadCanMutateConcurrently(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	previous, previousDigests := fixture.fullIndex(t)
	if _, _, err := runtimeFromIndexWithSourceDigests(previous, previousDigests, newSourceCache(), false); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.changed, `public class ChangedOwner { public static Integer value() { return 2; } }`)
	current := fixture.incrementalIndex(t, previous, []string{fixture.changed}, nil)
	affected, ok := runtimePatchOneModifiedOwner(previous, current)
	if !ok {
		t.Fatal("safe transition did not produce affected closure")
	}
	currentKey, returned, outcome, err := runtimeFromIndexTransition(previous, current, nil, newSourceCache(), false, nil, affected)
	if err != nil || !outcome.Applied {
		t.Fatalf("transition = %#v, err %v", outcome, err)
	}
	methodKey := ""
	for key, method := range returned.Methods {
		if method.ClassName == "ChangedOwner" {
			methodKey = key
			break
		}
	}
	if methodKey == "" {
		t.Fatal("changed method not found")
	}
	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-start
		for i := 0; i < 1000; i++ {
			method := returned.Methods[methodKey]
			method.Program.Source = fmt.Sprintf("caller-mutation-%d", i)
			returned.Methods[methodKey] = method
		}
	}()
	close(start)
	for i := 0; i < 32; i++ {
		if cached, ok := validMemoryRuntimeEntry(currentKey); !ok || cached.patchAuthority == nil {
			t.Fatal("caller-owned mutation invalidated cache-owned payload")
		}
	}
	<-done
}

func TestValidMemoryRuntimeEntryEvictsUncloneablePayload(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	index, digests := fixture.fullIndex(t)
	key, _, err := runtimeFromIndexWithSourceDigests(index, digests, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	runtimeCacheMu.Lock()
	cached := runtimeCache[key]
	mutated := false
	for methodKey, method := range cached.Methods {
		cycle := ir.Expr{Kind: ir.ExprUnary, Operator: "-"}
		cycle.Left = &cycle
		method.Program.Instructions = []ir.Instruction{{Op: ir.OpReturn, Expr: cycle}}
		cached.Methods[methodKey] = method
		mutated = true
		break
	}
	cached.patchAuthority = nil
	runtimeCache[key] = cached
	runtimeCacheMu.Unlock()
	if !mutated {
		t.Fatal("fixture runtime has no compiled method")
	}
	if _, ok := cloneRuntimeCacheEntryChecked(cached); ok {
		t.Fatal("recursive clone accepted cyclic IR")
	}
	if _, ok := validMemoryRuntimeEntry(key); ok {
		t.Fatal("cache returned an uncloneable payload as a valid hit")
	}
	runtimeCacheMu.RLock()
	_, exists := runtimeCache[key]
	runtimeCacheMu.RUnlock()
	if exists {
		t.Fatal("cache retained uncloneable payload after failed hit")
	}
}

func TestRuntimePatchCloneProgramAcceptsDeepAcyclicCompiledExpression(t *testing.T) {
	source := "String value = " + strings.Repeat("'segment' + ", 600) + "'tail';"
	program, err := vm.CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	cloned, ok := runtimePatchCloneProgram(program, make(map[*ir.Expr]bool), 0)
	if !ok {
		t.Fatal("runtime patch clone rejected valid deep acyclic IR")
	}
	if !reflect.DeepEqual(cloned, program) {
		t.Fatal("runtime patch clone changed valid deep acyclic IR")
	}
}

func TestRuntimePatchBaseAuthorityRequiresExactRuntimeInputsAndCleanErrors(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	index, digests := fixture.fullIndex(t)
	key, entry, err := runtimeFromIndexWithSourceDigests(index, digests, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, ok := runtimePatchIndexFingerprint(index)
	if !ok || entry.patchAuthority == nil {
		t.Fatal("fixture runtime lacks patch authority")
	}
	runtimeInputsFingerprint := entry.patchAuthority.runtimeInputsFingerprint
	if !runtimePatchBaseEntryTrusted(entry, key, fingerprint, runtimeInputsFingerprint) {
		t.Fatal("exact clean base authority was rejected")
	}

	tests := []struct {
		name   string
		mutate func(*runtimeCacheEntry)
	}{
		{
			name: "runtime inputs",
			mutate: func(candidate *runtimeCacheEntry) {
				copied := *candidate.patchAuthority
				copied.runtimeInputsFingerprint = "different"
				candidate.patchAuthority = &copied
			},
		},
		{name: "base error", mutate: func(candidate *runtimeCacheEntry) { candidate.BaseErr = errors.New("base") }},
		{name: "trigger errors", mutate: func(candidate *runtimeCacheEntry) { candidate.TriggerErrors = []error{errors.New("trigger")} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := entry
			test.mutate(&candidate)
			if runtimePatchBaseEntryTrusted(candidate, key, fingerprint, runtimeInputsFingerprint) {
				t.Fatal("mismatched or error-bearing base authority was trusted")
			}
		})
	}
}

type runtimeTransitionFixture struct {
	root    string
	changed string
	stable  string
	trigger string
}

func newRuntimeTransitionFixture(t *testing.T) runtimeTransitionFixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	changed := filepath.Join(root, "force-app/main/default/classes/ChangedOwner.cls")
	stable := filepath.Join(root, "force-app/main/default/classes/StableOwner.cls")
	trigger := filepath.Join(root, "force-app/main/default/triggers/TransitionTrigger.trigger")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, changed, `public class ChangedOwner { public static Integer value() { return 1; } }`)
	writeFile(t, stable, `public class StableOwner { public static Integer value() { return 7; } }`)
	writeFile(t, trigger, `trigger TransitionTrigger on Account (before insert) {}`)
	return runtimeTransitionFixture{root: root, changed: changed, stable: stable, trigger: trigger}
}

func (fixture runtimeTransitionFixture) fullIndex(t *testing.T) (typesys.Index, *typesys.SourceDigestSet) {
	t.Helper()
	loadedProject, err := project.Load(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	loadedSchema, err := gladeschema.LoadProject(loadedProject)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(loadedProject, loadedSchema)
	return index, artifacts.SourceDigests
}

func (fixture runtimeTransitionFixture) incrementalIndex(t *testing.T, previous typesys.Index, changed, deleted []string) typesys.Index {
	t.Helper()
	loadedProject, err := project.Load(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	current, exact, err := typesys.TryUpdateApexFilesCheckedWithLoadedProject(previous, changed, deleted, loadedProject)
	if err != nil {
		t.Fatal(err)
	}
	if !exact {
		t.Fatalf("fixture update was not exact: changed=%v deleted=%v", changed, deleted)
	}
	return current
}

func transitionMethod(t *testing.T, methods map[string]vm.Method, className, methodName string) vm.Method {
	t.Helper()
	for _, method := range methods {
		if method.ClassName == className && strings.EqualFold(method.Name, className+"."+methodName) {
			return method
		}
	}
	t.Fatalf("method %s.%s not found", className, methodName)
	return vm.Method{}
}

func runtimeTransitionContainsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func runtimeTransitionContainsExact(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sameTransitionInstructions(left, right vm.Method) bool {
	if len(left.Program.Instructions) == 0 || len(right.Program.Instructions) == 0 {
		return false
	}
	return &left.Program.Instructions[0] == &right.Program.Instructions[0]
}

func runtimeTransitionRequireNoReusedSourceInstructions(t *testing.T, previous, current runtimeCacheEntry) {
	t.Helper()
	checked := 0
	for key, previousMethod := range previous.Methods {
		currentMethod, ok := current.Methods[key]
		if !ok || previousMethod.File == "" || currentMethod.File == "" {
			continue
		}
		checked++
		if sameTransitionInstructions(previousMethod, currentMethod) {
			t.Fatalf("full fallback reused source instructions for %s", previousMethod.Name)
		}
	}
	if checked < 1 {
		t.Fatal("fallback instruction comparison covered no source methods")
	}
}

func runtimeTransitionErrorIdentities(errs []error) []string {
	out := make([]string, len(errs))
	for i, err := range errs {
		out[i] = runtimeTransitionErrorIdentity(err)
	}
	return out
}

func runtimeTransitionErrorIdentity(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T: %v", err, err)
}

func runtimeTransitionCompiledJSON(t *testing.T, entry runtimeCacheEntry) string {
	t.Helper()
	return runtimeTransitionJSON(t, compiledProjectRuntimeFromEntry(entry))
}

func runtimeTransitionJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runtimeTransitionCanonicalJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(runtimeTransitionCanonicalValue(decoded))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runtimeTransitionCanonicalValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			value[key] = runtimeTransitionCanonicalValue(child)
		}
		return value
	case []any:
		for i := range value {
			value[i] = runtimeTransitionCanonicalValue(value[i])
		}
		sort.Slice(value, func(i, j int) bool {
			left, _ := json.Marshal(value[i])
			right, _ := json.Marshal(value[j])
			return string(left) < string(right)
		})
		return value
	default:
		return value
	}
}

func runtimeTransitionFirstDifference(got, want string) string {
	limit := len(got)
	if len(want) < limit {
		limit = len(want)
	}
	index := 0
	for index < limit && got[index] == want[index] {
		index++
	}
	start := index - 80
	if start < 0 {
		start = 0
	}
	gotEnd := index + 160
	if gotEnd > len(got) {
		gotEnd = len(got)
	}
	wantEnd := index + 160
	if wantEnd > len(want) {
		wantEnd = len(want)
	}
	return "at " + fmt.Sprint(index) + " got=" + got[start:gotEnd] + " want=" + want[start:wantEnd]
}

package storage

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// referenceObjectNameCache stands in for the resolved-name cache of the
// reference resolver. Orgs that share a sync.Map share one of these.
type referenceObjectNameCache struct {
	names map[string]string
}

// referenceResolveObjectName is ResolveObjectName before the name index, with
// one change: it visits objects in sorted order instead of map order, so its
// first case-insensitive match is the smallest name, as the index picks.
func referenceResolveObjectName(org OrgState, cache *referenceObjectNameCache, name string) (string, bool) {
	cacheKey := strings.ToLower(strings.TrimSpace(org.Namespace)) + "|" + strconv.Itoa(len(org.Objects)) + "|" + strings.ToLower(strings.TrimSpace(name))
	if cache != nil {
		if resolved, ok := cache.names[cacheKey]; ok && resolved != "" {
			return resolved, true
		}
	}
	store := func(resolved string) (string, bool) {
		if cache != nil && resolved != "" {
			cache.names[cacheKey] = resolved
		}
		return resolved, true
	}
	sorted := make([]string, 0, len(org.Objects))
	for candidate := range org.Objects {
		sorted = append(sorted, candidate)
	}
	sort.Strings(sorted)
	if org.Namespace != "" && !hasNamespaceToken(name) && isCustomAPIName(name) {
		prefixed := NamespaceTokenName(org.Namespace, name)
		if prefixed != name {
			if _, ok := org.Objects[prefixed]; ok {
				return store(prefixed)
			}
		}
	}
	if exact, ok := org.Objects[name]; ok {
		if preferred, preferredOK := referenceRicherNamespacedObjectMatch(org, sorted, name, exact); preferredOK {
			return store(preferred)
		}
		return store(name)
	}
	prefixed := NamespaceTokenName(org.Namespace, name)
	if prefixed != name {
		if _, ok := org.Objects[prefixed]; ok {
			return store(prefixed)
		}
	}
	stripped := StripNamespaceToken(org.Namespace, name)
	if stripped != name {
		if _, ok := org.Objects[stripped]; ok {
			return store(stripped)
		}
	}
	for _, candidate := range sorted {
		if strings.EqualFold(candidate, name) || strings.EqualFold(candidate, prefixed) || strings.EqualFold(candidate, stripped) {
			return store(candidate)
		}
	}
	if hasNamespaceToken(name) {
		unqualified := StripAnyNamespaceToken(name)
		if unqualified != name {
			if _, ok := org.Objects[unqualified]; ok {
				return store(unqualified)
			}
			for _, candidate := range sorted {
				if strings.EqualFold(candidate, unqualified) {
					return store(candidate)
				}
			}
		}
	}
	if !hasNamespaceToken(name) && isCustomAPIName(name) {
		var match string
		for _, candidate := range sorted {
			if strings.EqualFold(StripAnyNamespaceToken(candidate), name) {
				if match != "" {
					return "", false
				}
				match = candidate
			}
		}
		if match != "" {
			return store(match)
		}
	}
	return "", false
}

func referenceRicherNamespacedObjectMatch(org OrgState, sorted []string, name string, exact ObjectState) (string, bool) {
	if hasNamespaceToken(name) || !isCustomAPIName(name) {
		return "", false
	}
	if prefixed := NamespaceTokenName(org.Namespace, name); prefixed != name {
		if state, ok := org.Objects[prefixed]; ok && objectDefinitionRicher(state.Definition, exact.Definition) {
			return prefixed, true
		}
	}
	var match string
	var matched ObjectState
	for _, candidate := range sorted {
		state := org.Objects[candidate]
		if strings.EqualFold(candidate, name) || !strings.EqualFold(StripAnyNamespaceToken(candidate), name) {
			continue
		}
		if match != "" {
			return "", false
		}
		match = candidate
		matched = state
	}
	if match == "" {
		return "", false
	}
	if objectDefinitionRicher(matched.Definition, exact.Definition) {
		return match, true
	}
	return "", false
}

// objectNameHandle pairs an org with the reference cache that mirrors its
// name cache.
type objectNameHandle struct {
	org OrgState
	ref *referenceObjectNameCache
}

func newReferenceObjectNameCache() *referenceObjectNameCache {
	return &referenceObjectNameCache{names: map[string]string{}}
}

var objectNameTestStems = []string{"Account", "Contact", "Thing", "Widget", "KelvinK", "Straße", "Longſs", "İd", "\xffBad"}
var objectNameTestNamespaces = []string{"", "pkg", "PKG", "other"}
var objectNameTestSuffixes = []string{"", "__c", "__C", "__Share", "__mdt", "__e", "__r"}

func randomObjectNameForTest(rng *rand.Rand) string {
	stem := objectNameTestStems[rng.Intn(len(objectNameTestStems))]
	switch rng.Intn(4) {
	case 0:
		stem = strings.ToLower(stem)
	case 1:
		stem = strings.ToUpper(stem)
	}
	name := stem + objectNameTestSuffixes[rng.Intn(len(objectNameTestSuffixes))]
	if ns := objectNameTestNamespaces[rng.Intn(len(objectNameTestNamespaces))]; ns != "" && rng.Intn(2) == 0 {
		name = ns + "__" + name
	}
	if rng.Intn(20) == 0 {
		name = " " + name
	}
	return name
}

func randomObjectDefinitionForTest(rng *rand.Rand, name string) ObjectDefinition {
	definition := ObjectDefinition{APIName: name, Fields: map[string]Field{}}
	for i := rng.Intn(3); i > 0; i-- {
		field := fmt.Sprintf("F%d__c", i)
		definition.Fields[field] = Field{APIName: field}
	}
	return definition
}

// Every lookup after every kind of org change returns what the resolver
// without the index returns: adding objects, DeleteObject, changing the
// namespace, rewriting a definition in place, each clone function, template
// clones, rollback snapshots and their restore, and a value copy whose map
// diverges while it shares the cache. The reference keeps a separate resolved-
// name cache wherever the org gets a fresh one.
func TestResolveObjectNameMatchesReferenceAcrossOrgChanges(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for round := 0; round < 300; round++ {
		org := NewOrgState()
		if round%3 == 0 {
			org.objectNameCache = nil
		}
		org.Namespace = objectNameTestNamespaces[rng.Intn(len(objectNameTestNamespaces))]
		for i := rng.Intn(12); i > 0; i-- {
			name := randomObjectNameForTest(rng)
			org.Objects[name] = ObjectState{Definition: randomObjectDefinitionForTest(rng, name)}
		}
		var ref *referenceObjectNameCache
		if org.objectNameCache != nil {
			ref = newReferenceObjectNameCache()
		}
		handles := []objectNameHandle{{org: org, ref: ref}}
		for step := 0; step < 120; step++ {
			h := rng.Intn(len(handles))
			handle := &handles[h]
			switch op := rng.Intn(22); {
			case op < 12:
				name := randomObjectNameForTest(rng)
				got, gotOK := ResolveObjectName(handle.org, name)
				want, wantOK := referenceResolveObjectName(handle.org, handle.ref, name)
				if got != want || gotOK != wantOK {
					t.Fatalf("round %d step %d: ResolveObjectName(%q) = %q, %v; want %q, %v (namespace %q, objects %v)", round, step, name, got, gotOK, want, wantOK, handle.org.Namespace, sortedObjectNames(handle.org))
				}
			case op == 12:
				name := randomObjectNameForTest(rng)
				handle.org.Objects[name] = ObjectState{Definition: randomObjectDefinitionForTest(rng, name)}
			case op == 13:
				names := sortedObjectNames(handle.org)
				if len(names) == 0 {
					continue
				}
				DeleteObject(&handle.org, names[rng.Intn(len(names))])
				if handle.ref != nil {
					clear(handle.ref.names)
				}
			case op == 14:
				handle.org.Namespace = objectNameTestNamespaces[rng.Intn(len(objectNameTestNamespaces))]
			case op == 15:
				names := sortedObjectNames(handle.org)
				if len(names) == 0 {
					continue
				}
				name := names[rng.Intn(len(names))]
				handle.org.Objects[name] = ObjectState{Definition: randomObjectDefinitionForTest(rng, name)}
			case op == 16:
				// A value copy with its own map that shares the cache, as
				// soql's lazy standard-object org does.
				fork := handle.org
				fork.Objects = make(map[string]ObjectState, len(handle.org.Objects)+1)
				for name, state := range handle.org.Objects {
					fork.Objects[name] = state
				}
				name := randomObjectNameForTest(rng)
				fork.Objects[name] = ObjectState{Definition: randomObjectDefinitionForTest(rng, name)}
				handles = append(handles, objectNameHandle{org: fork, ref: handle.ref})
			case op == 20:
				// A rollback snapshot, restored in place of the live org.
				snapshot := SnapshotRuntimeOrg(&handle.org)
				name := randomObjectNameForTest(rng)
				handle.org.Objects[name] = ObjectState{Definition: randomObjectDefinitionForTest(rng, name)}
				if rng.Intn(2) == 0 {
					handle.org = snapshot
					if handle.org.objectNameCache != nil {
						handle.ref = newReferenceObjectNameCache()
					}
				}
			case op == 21:
				snapshot := RuntimeTemplate{Org: handle.org}.CloneSnapshotOrg()
				var snapshotRef *referenceObjectNameCache
				if snapshot.objectNameCache != nil {
					snapshotRef = newReferenceObjectNameCache()
				}
				handles = append(handles, objectNameHandle{org: snapshot, ref: snapshotRef})
			default:
				var clone OrgState
				switch op {
				case 17:
					clone = handle.org.Clone()
				case 18:
					clone = handle.org.CloneRuntimeFrozenDefinition()
				default:
					if rng.Intn(2) == 0 {
						clone = handle.org.CloneRuntimeFrozenShared()
					} else {
						clone = RuntimeTemplate{Org: handle.org}.CloneRuntimeOrg()
					}
				}
				var cloneRef *referenceObjectNameCache
				if clone.objectNameCache != nil {
					cloneRef = newReferenceObjectNameCache()
				}
				handles = append(handles, objectNameHandle{org: clone, ref: cloneRef})
			}
			if len(handles) > 6 {
				handles = handles[len(handles)-6:]
			}
		}
	}
}

func sortedObjectNames(org OrgState) []string {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestObjectNameFoldKeyMatchesEqualFold(t *testing.T) {
	runes := []string{"a", "A", "k", "K", "K", "s", "S", "ſ", "ß", "ẞ", "i", "I", "İ", "ı", "Σ", "σ", "ς", "_", "1", "\xff", "�", "Ǆ", "ǅ", "ǆ"}
	rng := rand.New(rand.NewSource(3))
	random := func() string {
		var b strings.Builder
		for i := rng.Intn(4); i >= 0; i-- {
			b.WriteString(runes[rng.Intn(len(runes))])
		}
		return b.String()
	}
	for i := 0; i < 50000; i++ {
		left, right := random(), random()
		if got, want := objectNameFoldKey(left) == objectNameFoldKey(right), strings.EqualFold(left, right); got != want {
			t.Fatalf("fold keys of %q and %q equal = %v; strings.EqualFold = %v", left, right, got, want)
		}
	}
}

// The index is built once per Objects map and carried to clones, and a name
// that matches nothing is answered from the miss cache. Without the index the
// build counter stays at zero; without carrying, clones rebuild.
func TestResolveObjectNameBuildsIndexOnceAndCachesMisses(t *testing.T) {
	org := NewOrgState()
	org.Namespace = "pkg"
	for i := 0; i < 2000; i++ {
		name := fmt.Sprintf("pkg__Object%d__c", i)
		org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name}}
	}
	before := objectNameIndexBuilds.Load()
	for i := 0; i < 2000; i++ {
		if resolved, ok := ResolveObjectName(org, fmt.Sprintf("OBJECT%d__C", i)); !ok || resolved != fmt.Sprintf("pkg__Object%d__c", i) {
			t.Fatalf("ResolveObjectName(OBJECT%d__C) = %q, %v", i, resolved, ok)
		}
		if resolved, ok := ResolveObjectName(org, fmt.Sprintf("Missing%d__c", i)); ok {
			t.Fatalf("ResolveObjectName(Missing%d__c) = %q, true", i, resolved)
		}
	}
	if builds := objectNameIndexBuilds.Load() - before; builds != 1 {
		t.Fatalf("index builds = %d for 4000 lookups on one org; want 1", builds)
	}
	index := objectNameIndexFor(org)
	if _, ok := index.misses.Load(objectNameMiss{namespace: "pkg", name: "Missing7__c"}); !ok {
		t.Fatal("miss for Missing7__c was not cached")
	}
	clones := map[string]OrgState{
		"Clone":                            org.Clone(),
		"CloneRuntime":                     org.CloneRuntime(),
		"CloneRuntimeFrozenDefinition":     org.CloneRuntimeFrozenDefinition(),
		"CloneRuntimeFrozenShared":         org.CloneRuntimeFrozenShared(),
		"RuntimeTemplate.CloneRuntimeOrg":  RuntimeTemplate{Org: org}.CloneRuntimeOrg(),
		"RuntimeTemplate.CloneSnapshotOrg": RuntimeTemplate{Org: org}.CloneSnapshotOrg(),
		"SnapshotRuntimeOrg":               SnapshotRuntimeOrg(&org),
	}
	for name, clone := range clones {
		if resolved, ok := ResolveObjectName(clone, "object7__C"); !ok || resolved != "pkg__Object7__c" {
			t.Fatalf("%s: ResolveObjectName(object7__C) = %q, %v", name, resolved, ok)
		}
		if _, ok := ResolveObjectName(clone, "Missing7__c"); ok {
			t.Fatalf("%s: Missing7__c resolved", name)
		}
	}
	if builds := objectNameIndexBuilds.Load() - before; builds != 1 {
		t.Fatalf("index builds = %d after seven clones; want 1 (clones carry the index)", builds)
	}

	// A clone that adds objects extends the carried index.
	clone := org.CloneRuntimeFrozenDefinition()
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("Added%d__c", i)
		clone.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name}}
		if resolved, ok := ResolveObjectName(clone, strings.ToUpper(name)); !ok || resolved != name {
			t.Fatalf("ResolveObjectName(%s) = %q, %v", strings.ToUpper(name), resolved, ok)
		}
		if resolved, ok := ResolveObjectName(clone, "OBJECT9__C"); !ok || resolved != "pkg__Object9__c" {
			t.Fatalf("after add: ResolveObjectName(OBJECT9__C) = %q, %v", resolved, ok)
		}
	}
	if builds := objectNameIndexBuilds.Load() - before; builds != 1 {
		t.Fatalf("index builds = %d after additions to a clone; want 1 (additions extend the index)", builds)
	}
}

func TestResolveObjectNameIndexFollowsObjectChanges(t *testing.T) {
	org := NewOrgState()
	org.Objects["Alpha__c"] = ObjectState{}
	org.Objects["Beta__c"] = ObjectState{}
	if _, ok := ResolveObjectName(org, "gamma__C"); ok {
		t.Fatal("gamma__C resolved before it exists")
	}

	// Adding an object changes the count.
	org.Objects["Gamma__c"] = ObjectState{}
	if resolved, ok := ResolveObjectName(org, "gamma__C"); !ok || resolved != "Gamma__c" {
		t.Fatalf("after add: ResolveObjectName(gamma__C) = %q, %v", resolved, ok)
	}

	// Deleting one object and adding another keeps the count; DeleteObject
	// drops the cache.
	if _, ok := ResolveObjectName(org, "delta__C"); ok {
		t.Fatal("delta__C resolved before it exists")
	}
	DeleteObject(&org, "Alpha__c")
	org.Objects["Delta__c"] = ObjectState{}
	if resolved, ok := ResolveObjectName(org, "delta__C"); !ok || resolved != "Delta__c" {
		t.Fatalf("after delete and add: ResolveObjectName(delta__C) = %q, %v", resolved, ok)
	}
	if resolved, ok := ResolveObjectName(org, "alpha__C"); ok {
		t.Fatalf("after delete: ResolveObjectName(alpha__C) = %q, true", resolved)
	}

	// A copy with its own map shares the cache but not the index. (Resolved
	// names are shared by namespace and count, as before the index, so the
	// fork has a different count.)
	fork := org
	fork.Objects = map[string]ObjectState{"Epsilon__c": {}, "Beta__c": {}, "Gamma__c": {}, "Theta__c": {}}
	if resolved, ok := ResolveObjectName(fork, "epsilon__C"); !ok || resolved != "Epsilon__c" {
		t.Fatalf("fork: ResolveObjectName(epsilon__C) = %q, %v", resolved, ok)
	}
	if _, ok := ResolveObjectName(org, "epsilon__C"); ok {
		t.Fatal("org resolved the fork's object")
	}

	// A namespace change is part of the miss key.
	org.Objects["pkg__Zeta__c"] = ObjectState{}
	if _, ok := ResolveObjectName(org, "ZETA__C"); !ok {
		t.Fatal("ZETA__C did not resolve to the only namespaced match")
	}
	org.Objects["other__Zeta__c"] = ObjectState{}
	if _, ok := ResolveObjectName(org, "ZETA__C"); ok {
		t.Fatal("ZETA__C resolved although two namespaces match")
	}
	org.Namespace = "pkg"
	if resolved, ok := ResolveObjectName(org, "ZETA__C"); !ok || resolved != "pkg__Zeta__c" {
		t.Fatalf("after namespace change: ResolveObjectName(ZETA__C) = %q, %v", resolved, ok)
	}

	// A clone that adds an object does not change its source.
	clone := org.CloneRuntimeFrozenDefinition()
	clone.Objects["Eta__c"] = ObjectState{}
	if resolved, ok := ResolveObjectName(clone, "eta__C"); !ok || resolved != "Eta__c" {
		t.Fatalf("clone: ResolveObjectName(eta__C) = %q, %v", resolved, ok)
	}
	if _, ok := ResolveObjectName(org, "eta__C"); ok {
		t.Fatal("source resolved the clone's object")
	}
}

func TestResolveObjectNameConcurrentLookupsShareCache(t *testing.T) {
	org := NewOrgState()
	for i := 0; i < 500; i++ {
		name := fmt.Sprintf("Object%d__c", i)
		org.Objects[name] = ObjectState{}
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			clone := org
			if worker%2 == 1 {
				clone = org.CloneRuntimeFrozenDefinition()
			}
			for i := 0; i < 500; i++ {
				if resolved, ok := ResolveObjectName(clone, fmt.Sprintf("OBJECT%d__C", (i+worker)%500)); !ok || resolved != fmt.Sprintf("Object%d__c", (i+worker)%500) {
					t.Errorf("ResolveObjectName = %q, %v", resolved, ok)
					return
				}
				if _, ok := ResolveObjectName(clone, fmt.Sprintf("Missing%d__c", i)); ok {
					t.Error("missing object resolved")
					return
				}
			}
		}(worker)
	}
	wg.Wait()
}

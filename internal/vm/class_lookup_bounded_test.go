package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestFrozenClassLookupUsesBoundedResultOverlay(t *testing.T) {
	template := New(nil)
	if err := template.RegisterClass(Class{Name: "Worker", Namespace: "pkg", Dependency: true}); err != nil {
		t.Fatal(err)
	}
	template.FreezeClassLookup()
	clone := template.CloneRuntimeFrozenShared(nil)

	for range 2 {
		class, ok := clone.lookupClass("PKG.WORKER")
		if !ok || runtimeClassName(class) != "pkg.Worker" {
			t.Fatalf("frozen lookup = %+v, %v", class, ok)
		}
	}
	if len(clone.classLookupNameCache) != 1 {
		t.Fatalf("frozen lookup overlay entries = %d, want 1", len(clone.classLookupNameCache))
	}
	if clone.classLookupNameStats.Hits != 1 || clone.classLookupNameStats.Misses != 1 {
		t.Fatalf("frozen lookup overlay stats = %+v, want one miss then one hit", clone.classLookupNameStats)
	}
	if clone.classLookupNameStats.Entries > maxClassLookupNameCacheEntries ||
		clone.classLookupNameStats.RetainedBytes > maxClassLookupNameCacheBytes {
		t.Fatalf("frozen lookup overlay exceeded bounds: %+v", clone.classLookupNameStats)
	}
}

func TestClassLookupPerfCountersUseDistinctRuntimeShards(t *testing.T) {
	if testing.Short() {
		t.Skip("infrastructure test; full suite runs in acceptance lanes")
	}

	template := New(nil)
	if err := template.RegisterClass(Class{Name: "Worker"}); err != nil {
		t.Fatal(err)
	}
	template.FreezeClassLookup()
	recorder := NewPerfRecorder()
	template.SetPerfRecorder(recorder)
	first := template.CloneRuntimeFrozenShared(nil)
	second := template.CloneRuntimeFrozenShared(nil)

	firstShard := classLookupPerfShardPointer(first)
	secondShard := classLookupPerfShardPointer(second)
	if firstShard == 0 || secondShard == 0 || firstShard == secondShard {
		t.Fatalf("runtime class-lookup perf shards = %#x, %#x; want distinct nonzero shards", firstShard, secondShard)
	}

	for range 2 {
		if _, ok := first.lookupClass("Worker"); !ok {
			t.Fatal("first runtime lookup missed")
		}
		if _, ok := second.lookupClass("Worker"); !ok {
			t.Fatal("second runtime lookup missed")
		}
	}
	perf := recorder.Snapshot().ClassLookup
	if perf.Hits != 2 || perf.Misses != 2 {
		t.Fatalf("sharded class lookup perf = %+v, want two hits and two misses", perf)
	}
}

func TestClassLookupPerfShardPoolIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("infrastructure test; full suite runs in acceptance lanes")
	}

	template := New(nil)
	if err := template.RegisterClass(Class{Name: "Worker"}); err != nil {
		t.Fatal(err)
	}
	template.FreezeClassLookup()
	recorder := NewPerfRecorder()
	template.SetPerfRecorder(recorder)

	shards := make(map[uintptr]struct{})
	for range 128 {
		clone := template.CloneRuntimeFrozenShared(nil)
		pointer := classLookupPerfShardPointer(clone)
		if pointer == 0 {
			t.Fatal("runtime class-lookup perf shard is nil")
		}
		shards[pointer] = struct{}{}
		for range 2 {
			if _, ok := clone.lookupClass("Worker"); !ok {
				t.Fatal("runtime class lookup missed")
			}
		}
	}
	if len(shards) != 64 {
		t.Fatalf("runtime class-lookup perf shards = %d, want bounded pool of 64", len(shards))
	}
	perf := recorder.Snapshot().ClassLookup
	if perf.Hits != 128 || perf.Misses != 128 {
		t.Fatalf("reused shard accounting = %+v, want 128 hits and 128 misses", perf)
	}
}

func classLookupPerfShardPointer(machine *VM) uintptr {
	field := reflect.ValueOf(machine).Elem().FieldByName("classLookupPerf")
	if !field.IsValid() || field.IsNil() {
		return 0
	}
	return field.Pointer()
}

func TestFrozenClassLookupCacheIsGenerationBoundAndBounded(t *testing.T) {
	template := New(nil)
	for _, class := range []Class{
		{Name: "Local"},
		{Name: "Managed", Namespace: "pkg", Dependency: true},
		{Name: "Outer.Inner", Namespace: "pkg", Dependency: true},
		{Name: "Collision"},
		{Name: "Collision", Namespace: "pkg", Dependency: true},
		{Name: "LateCollision", Namespace: "pkg", Dependency: true},
	} {
		if err := template.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	template.FreezeClassLookup()
	if template.frozenClassLookup == nil || template.frozenClassLookup.generation == 0 {
		t.Fatal("frozen class lookup has no generation")
	}

	clone := template.CloneRuntimeFrozenShared(nil)
	sibling := template.CloneRuntimeFrozenShared(nil)
	if clone.frozenClassLookup != template.frozenClassLookup {
		t.Fatal("clone did not share the exact frozen lookup generation")
	}
	recorder := NewPerfRecorder()
	clone.SetPerfRecorder(recorder)
	for _, name := range []string{"Local", "local", "pkg.Managed", "PKG.MANAGED", "pkg.Outer.Inner"} {
		first, ok := clone.lookupClass(name)
		if !ok {
			t.Fatalf("lookupClass(%q) = miss, want hit", name)
		}
		second, ok := clone.lookupClass(name)
		if !ok || runtimeClassName(second) != runtimeClassName(first) {
			t.Fatalf("cached lookupClass(%q) = %q, want %q", name, runtimeClassName(second), runtimeClassName(first))
		}
	}
	localCollision, ok := clone.lookupClass("Collision")
	if !ok || localCollision.Dependency {
		t.Fatalf("unqualified collision = %+v, want local class", localCollision)
	}
	managedCollision, ok := clone.lookupClass("pkg.Collision")
	if !ok || !managedCollision.Dependency {
		t.Fatalf("qualified collision = %+v, want managed class", managedCollision)
	}
	if _, ok := clone.lookupClass("MissingFrozen"); ok {
		t.Fatal("frozen lookup unexpectedly resolved a missing class")
	}
	if len(clone.classLookupNameCache) == 0 ||
		clone.classLookupNameStats.Entries > maxClassLookupNameCacheEntries ||
		clone.classLookupNameStats.RetainedBytes > maxClassLookupNameCacheBytes {
		t.Fatalf("stable frozen lookup overlay is missing or unbounded: %+v", clone.classLookupNameStats)
	}

	frozenGeneration := clone.frozenClassLookup.generation
	if err := clone.RegisterClass(Class{Name: "Added"}); err != nil {
		t.Fatal(err)
	}
	if clone.frozenClassLookup != nil || clone.classLookupGeneration == frozenGeneration {
		t.Fatal("post-freeze registration retained stale frozen generation")
	}
	if _, ok := clone.lookupClass("added"); !ok {
		t.Fatal("post-freeze registered class is not visible through private overlay")
	}
	if _, ok := sibling.lookupClass("Added"); ok {
		t.Fatal("post-freeze registration changed sibling frozen generation")
	}
	if sibling.frozenClassLookup != template.frozenClassLookup {
		t.Fatal("post-freeze registration detached sibling frozen lookup")
	}
	if _, ok := clone.lookupClass("AddedLater"); ok {
		t.Fatal("private overlay unexpectedly resolved class before registration")
	}
	privateGeneration := clone.classLookupGeneration
	if err := clone.RegisterClass(Class{Name: "AddedLater"}); err != nil {
		t.Fatal(err)
	}
	if clone.classLookupGeneration == privateGeneration {
		t.Fatal("second post-freeze registration retained stale private generation")
	}
	if _, ok := clone.lookupClass("addedlater"); !ok {
		t.Fatal("second post-freeze registration did not invalidate private miss")
	}
	if err := clone.RegisterClass(Class{Name: "LateCollision"}); err != nil {
		t.Fatal(err)
	}
	lateLocal, ok := clone.lookupClass("LateCollision")
	if !ok || lateLocal.Dependency {
		t.Fatalf("post-freeze unqualified collision = %+v, want local class", lateLocal)
	}
	lateManaged, ok := clone.lookupClass("pkg.LateCollision")
	if !ok || !lateManaged.Dependency {
		t.Fatalf("post-freeze qualified collision = %+v, want managed class", lateManaged)
	}
	siblingLate, ok := sibling.lookupClass("LateCollision")
	if !ok || !siblingLate.Dependency {
		t.Fatalf("sibling collision = %+v, want unchanged managed class", siblingLate)
	}

	for i := 0; i < maxClassLookupNameCacheEntries*4; i++ {
		clone.lookupClass(fmt.Sprintf("MissingOverlay%d", i))
	}
	longMissingPrefix := strings.Repeat("LongMissing", 32)
	for i := 0; i < maxClassLookupNameCacheEntries*2; i++ {
		clone.lookupClass(fmt.Sprintf("%s%d", longMissingPrefix, i))
	}
	stats := clone.classLookupNameStats
	if stats.Entries > maxClassLookupNameCacheEntries {
		t.Fatalf("entries = %d, limit %d", stats.Entries, maxClassLookupNameCacheEntries)
	}
	if stats.RetainedBytes > maxClassLookupNameCacheBytes {
		t.Fatalf("retained bytes = %d, limit %d", stats.RetainedBytes, maxClassLookupNameCacheBytes)
	}
	if stats.Evictions == 0 {
		t.Fatal("arbitrary misses did not evict bounded lookup entries")
	}
	clone.lookupClass("Added")
	clone.lookupClass("Added")
	if clone.classLookupNameStats.Hits == 0 || clone.classLookupNameStats.Misses == 0 {
		t.Fatalf("cache counters = %+v, want hits and misses", clone.classLookupNameStats)
	}
	perf := recorder.Snapshot().ClassLookup
	if perf.Hits == 0 || perf.Misses == 0 || perf.Entries == 0 || perf.Evictions == 0 || perf.RetainedBytes == 0 {
		t.Fatalf("class lookup perf counters incomplete: %+v", perf)
	}
}

func TestFreshRuntimeDoesNotRetainUnusableClassLookupOverlay(t *testing.T) {
	machine := New(nil)
	if _, ok := machine.lookupClass("MissingFreshRuntimeClass"); ok {
		t.Fatal("unexpected class lookup hit")
	}
	if machine.classLookupGeneration != 0 {
		t.Fatalf("fresh runtime generation = %d, want zero", machine.classLookupGeneration)
	}
	if len(machine.classLookupNameCache) != 0 || len(machine.classLookupNameOrder) != 0 || machine.classLookupNameBytes != 0 {
		t.Fatalf(
			"fresh runtime retained lookup overlay: entries=%d order=%d bytes=%d",
			len(machine.classLookupNameCache),
			len(machine.classLookupNameOrder),
			machine.classLookupNameBytes,
		)
	}
}

func BenchmarkFrozenClassLookupCache(b *testing.B) {
	template := New(nil)
	for i := 0; i < 2000; i++ {
		if err := template.RegisterClass(Class{Name: fmt.Sprintf("Class%d", i)}); err != nil {
			b.Fatal(err)
		}
	}
	template.FreezeClassLookup()
	machine := template.CloneRuntimeFrozenShared(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := machine.lookupClass("class1500"); !ok {
			b.Fatal("class lookup miss")
		}
	}
}

func TestFrozenNamespaceClassLookupMatchesUnsharedBuild(t *testing.T) {
	classes := []Class{
		{Name: "Worker", Namespace: "pkg", Dependency: true},
		{Name: "Worker", Namespace: "other", Dependency: true},
		{Name: "Worker"},
		{Name: "Outer"},
		{Name: "Outer.Inner"},
		{Name: "Shell"},
		{Name: "Shell.Inner"},
		{Name: "Registry", Namespace: "pkg", Dependency: true},
		{Name: "Registry.Entry", Namespace: "pkg", Dependency: true},
	}
	template := New(nil)
	for _, class := range classes {
		if err := template.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	template.FreezeClassLookup()
	first := template.CloneRuntimeFrozenShared(nil)
	second := template.CloneRuntimeFrozenShared(nil)
	// The reference clone drops the frozen generation and builds its
	// namespace tables through the unshared path.
	unshared := template.CloneRuntimeFrozenShared(nil)
	unshared.frozenClassLookup = nil
	if unshared.frozenClassLookup != nil || first.frozenClassLookup == nil || first.frozenClassLookup != second.frozenClassLookup {
		t.Fatalf("unexpected frozen lookup sharing")
	}
	for _, namespace := range []string{"", "pkg", "PKG", "other", "missing"} {
		for _, name := range []string{"Worker", "Inner", "Entry", "Registry", "Outer.Inner", "Missing"} {
			wantClass, wantOK := unshared.lookupClassInNamespace(namespace, name)
			for _, clone := range []*VM{first, second} {
				gotClass, gotOK := clone.lookupClassInNamespace(namespace, name)
				if gotOK != wantOK || !reflect.DeepEqual(gotClass, wantClass) {
					t.Fatalf("lookupClassInNamespace(%q, %q) = %+v, %v; want %+v, %v", namespace, name, gotClass, gotOK, wantClass, wantOK)
				}
			}
		}
		key := strings.ToLower(namespace)
		if !reflect.DeepEqual(first.namespaceClassLookup[key], unshared.namespaceClassLookup[key]) {
			t.Fatalf("namespace %q table = %+v, want %+v", namespace, first.namespaceClassLookup[key], unshared.namespaceClassLookup[key])
		}
	}
	if len(first.frozenClassLookup.namespaceAliases) == 0 {
		t.Fatalf("frozen namespace aliases were not shared")
	}

	// Registration on a clone drops the shared generation; the clone must see
	// its new class and the other clone must not.
	if err := second.RegisterClass(Class{Name: "Late", Namespace: "pkg", Dependency: true}); err != nil {
		t.Fatal(err)
	}
	if second.frozenClassLookup != nil {
		t.Fatalf("registration kept the frozen lookup")
	}
	if _, ok := second.lookupClassInNamespace("pkg", "Late"); !ok {
		t.Fatalf("registered class missing from namespace lookup")
	}
	third := template.CloneRuntimeFrozenShared(nil)
	if _, ok := third.lookupClassInNamespace("pkg", "Late"); ok {
		t.Fatalf("clone registration leaked into the shared namespace table")
	}
}

func TestFrozenNamespaceClassLookupCloneRegistrationKeepsBaseTable(t *testing.T) {
	base := New(nil)
	for _, class := range []Class{
		{Name: "Worker", Namespace: "pkg", Dependency: true},
		{Name: "Worker"},
	} {
		if err := base.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	base.FreezeClassLookup()
	frozen := base.frozenClassLookup

	resolve := func(machine *VM, namespace string) Class {
		t.Helper()
		class, ok := machine.lookupClassInNamespace(namespace, "Worker")
		if !ok {
			t.Fatalf("lookupClassInNamespace(%q, Worker) not found", namespace)
		}
		return class
	}
	clone := base.CloneRuntimeFrozenShared(nil)
	if got := resolve(clone, "pkg"); got.Namespace != "pkg" || !got.Dependency {
		t.Fatalf("pkg Worker = %+v, want the dependency class", got)
	}
	if got := resolve(clone, ""); got.Namespace != "" || got.Dependency {
		t.Fatalf("unqualified Worker = %+v, want the project class", got)
	}
	frozen.namespaceMu.RLock()
	before := make(map[string]map[string]namespaceClassAlias, len(frozen.namespaceAliases))
	for namespace, aliases := range frozen.namespaceAliases {
		copied := make(map[string]namespaceClassAlias, len(aliases))
		for key, alias := range aliases {
			copied[key] = alias
		}
		before[namespace] = copied
	}
	frozen.namespaceMu.RUnlock()
	if len(before["pkg"]) == 0 || len(before[""]) == 0 {
		t.Fatalf("shared namespace tables = %+v, want pkg and default", before)
	}

	if err := clone.RegisterClass(Class{Name: "Helper", Namespace: "pkg", Dependency: true}); err != nil {
		t.Fatal(err)
	}
	if clone.frozenClassLookup != nil {
		t.Fatal("registration kept the shared frozen lookup")
	}
	if _, ok := clone.lookupClassInNamespace("pkg", "Helper"); !ok {
		t.Fatal("clone does not resolve its registered class")
	}
	if got := resolve(clone, "pkg"); got.Namespace != "pkg" || !got.Dependency {
		t.Fatalf("pkg Worker after registration = %+v", got)
	}
	if got := resolve(clone, ""); got.Namespace != "" || got.Dependency {
		t.Fatalf("unqualified Worker after registration = %+v", got)
	}

	if base.frozenClassLookup != frozen {
		t.Fatal("clone registration replaced the base's frozen lookup")
	}
	frozen.namespaceMu.RLock()
	after := frozen.namespaceAliases
	frozen.namespaceMu.RUnlock()
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("base namespace tables changed: %+v, want %+v", after, before)
	}
	if _, ok := base.lookupClassInNamespace("pkg", "Helper"); ok {
		t.Fatal("clone registration leaked into the base")
	}
	if _, ok := base.CloneRuntimeFrozenShared(nil).lookupClassInNamespace("pkg", "Helper"); ok {
		t.Fatal("clone registration leaked into a later clone")
	}
}

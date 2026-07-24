package apextest

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestInvalidMemoryRuntimeEntryIsEvictedAndRebuilt(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/RestoredBoundaryTest.cls"), `
@isTest private class RestoredBoundaryTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
	index := loadTestIndex(t, root)
	key := runtimeKey(index)
	runtimeCacheMu.Lock()
	runtimeCache[key] = runtimeCacheEntry{}
	runtimeCacheMu.Unlock()

	counters := newRunPerfCounters(true)
	gotKey, entry, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), false, counters)
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != key {
		t.Fatalf("runtime key = %q, want %q", gotKey, key)
	}
	if !entry.restored.Valid() {
		t.Fatal("rebuilt runtime entry is invalid")
	}
	phases := snapshotPerfCounters(counters).Phases
	if phases.MemoryCacheHits != 0 || phases.CacheMisses != 1 {
		t.Fatalf("rebuild phases = %#v, want one miss and no memory hit", phases)
	}
}

func TestInvalidMemoryObservationRetainsConcurrentValidReplacement(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	key := runtimeCacheKey("concurrent-valid-replacement")
	runtimeCacheMu.Lock()
	runtimeCache[key] = runtimeCacheEntry{}
	runtimeCacheMu.Unlock()

	// Simulate the first read phase observing an invalid entry, followed by a
	// concurrent builder publishing a valid replacement before the write-lock
	// recheck.
	runtimeCacheMu.RLock()
	observed := runtimeCache[key]
	runtimeCacheMu.RUnlock()
	if observed.restored.Valid() {
		t.Fatal("first phase did not observe the injected invalid entry")
	}
	valid := runtimeCacheEntry{
		Methods: map[string]vm.Method{"Replacement.marker": {Name: "Replacement.marker"}},
		restored: vm.NewRestoredRuntimeTemplate(
			storage.NewOrgState(),
			vm.New(nil),
		),
	}
	runtimeCacheMu.Lock()
	runtimeCache[key] = valid
	runtimeCacheMu.Unlock()

	got, ok := recheckMemoryRuntimeEntryAfterInvalidObservation(key)
	if !ok || !got.restored.Valid() {
		t.Fatalf("valid replacement was not returned: ok=%v entry=%#v", ok, got)
	}
	if _, ok := got.Methods["Replacement.marker"]; !ok {
		t.Fatalf("returned entry is not the valid replacement: %#v", got.Methods)
	}
	runtimeCacheMu.RLock()
	retained := runtimeCache[key]
	runtimeCacheMu.RUnlock()
	if !retained.restored.Valid() {
		t.Fatal("valid replacement was evicted during write-lock recheck")
	}
}

func TestDiskRestoredRuntimeUsesOpaqueCloneBoundary(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() { disableDiskCache.Store(wasDisabled) })

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OpaqueBoundaryTest.cls"), `
@isTest private class OpaqueBoundaryTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
	index := loadTestIndex(t, root)
	key, built, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !built.restored.Valid() {
		t.Fatal("built runtime entry is invalid")
	}
	InvalidateRuntimeCaches()
	restored, ok := tryLoadDiskRuntimeWithSourceDigests(index, nil, key)
	if !ok {
		t.Fatal("disk runtime was not restored")
	}
	if !restored.restored.Valid() {
		t.Fatal("disk-restored runtime entry is invalid")
	}

	firstOrg := restored.restored.CloneOrg()
	firstOrg.OrgID = "changed"
	secondOrg := restored.restored.CloneOrg()
	if secondOrg.OrgID == "changed" {
		t.Fatal("disk-restored org clones share mutable identity")
	}
	firstMachine := restored.restored.CloneMachine(nil)
	if err := firstMachine.RegisterMethod(vm.Method{Name: "OpaqueBoundary.extra", ReturnType: "String"}); err != nil {
		t.Fatal(err)
	}
	secondMachine := restored.restored.CloneMachine(nil)
	if _, ok := secondMachine.Methods["OpaqueBoundary.extra"]; ok {
		t.Fatal("disk-restored machine clones share registered methods")
	}

	entryType := reflect.TypeOf(runtimeCacheEntry{})
	for i := 0; i < entryType.NumField(); i++ {
		field := entryType.Field(i)
		switch field.Type {
		case reflect.TypeOf(storage.OrgState{}), reflect.TypeOf(storage.RuntimeTemplate{}), reflect.TypeOf((*vm.VM)(nil)):
			t.Fatalf("runtimeCacheEntry exposes raw mutable field %s %s", field.Name, field.Type)
		}
	}
}

func TestInvalidRestoredRuntimeEntryIsRejected(t *testing.T) {
	entry, ok := validateRestoredRuntimeEntry(runtimeCacheEntry{})
	if ok {
		t.Fatalf("zero restored disk entry accepted: %#v", entry)
	}
}

func TestMalformedDiskRuntimeIsAMissAndRebuilds(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() { disableDiskCache.Store(wasDisabled) })

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/MalformedBoundaryTest.cls"), `
@isTest private class MalformedBoundaryTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
	index := loadTestIndex(t, root)
	if _, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), true); err != nil {
		t.Fatal(err)
	}
	InvalidateRuntimeCaches()
	headerPath := filepath.Join(root, ".glade", "test", "startup.meta.json")
	if err := os.WriteFile(headerPath, []byte("{malformed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	counters := newRunPerfCounters(true)
	key, rebuilt, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), true, counters)
	if err != nil {
		t.Fatal(err)
	}
	if !rebuilt.restored.Valid() {
		t.Fatal("runtime rebuilt from malformed disk entry is invalid")
	}
	phases := snapshotPerfCounters(counters).Phases
	if phases.DiskCacheHits != 0 || phases.MemoryCacheHits != 0 || phases.CacheMisses != 1 {
		t.Fatalf("malformed disk phases = %#v, want one miss and no cache hits", phases)
	}
	runtimeCacheMu.RLock()
	published, ok := runtimeCache[key]
	runtimeCacheMu.RUnlock()
	if !ok || !published.restored.Valid() {
		t.Fatalf("rebuilt runtime was not published validly: ok=%v entry=%#v", ok, published)
	}

	InvalidateRuntimeCaches()
	coldCounters := newRunPerfCounters(true)
	_, cold, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), true, coldCounters)
	if err != nil {
		t.Fatal(err)
	}
	if !cold.restored.Valid() {
		t.Fatal("runtime restored from repaired disk artifact is invalid")
	}
	coldPhases := snapshotPerfCounters(coldCounters).Phases
	if coldPhases.DiskCacheHits != 1 || coldPhases.MemoryCacheHits != 0 || coldPhases.CacheMisses != 0 {
		t.Fatalf("repaired disk phases = %#v, want one disk hit and no miss", coldPhases)
	}
}

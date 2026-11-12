package vm

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// frozenPrefixOrg returns a stamped org: its schema is frozen and shared by
// every clone, so the key-prefix index is built once per generation.
func frozenPrefixOrg(rng *rand.Rand, objects int) (storage.OrgState, []string) {
	org, prefixes, _ := idLookupOrg(rng, objects)
	org.RuntimeSchemaStamp = "test-stamp"
	return org, prefixes
}

func TestKeyPrefixIndexMatchesSortedScan(t *testing.T) {
	rng := rand.New(rand.NewSource(20261112))
	for round := 0; round < 60; round++ {
		org, prefixes := frozenPrefixOrg(rng, 1+rng.Intn(80))
		machine := New(nil)
		machine.Org = &org
		for pass := 0; pass < 2; pass++ {
			for _, prefix := range append(prefixes, "00Q", "xyz", "00") {
				wantName, wantOK := referenceObjectNameForKeyPrefix(&org, prefix)
				gotName, gotOK := machine.orgObjectNameForKeyPrefix(prefix)
				if gotName != wantName || gotOK != wantOK {
					t.Fatalf("round %d pass %d prefix %q: got %q, %v; want %q, %v", round, pass, prefix, gotName, gotOK, wantName, wantOK)
				}
			}
		}
	}
}

// Fails while every lookup walks all objects: on a frozen generation the
// index answers hits and misses without a pass.
func TestKeyPrefixLookupDoesNotScanFrozenGeneration(t *testing.T) {
	org, prefixes := frozenPrefixOrg(rand.New(rand.NewSource(7)), 200)
	machine := New(nil)
	machine.Org = &org
	machine.orgObjectNameForKeyPrefix(prefixes[1]) // builds the index
	before := keyPrefixScans.Load()
	for i := 0; i < 100; i++ {
		machine.orgObjectNameForKeyPrefix(prefixes[1+i%(len(prefixes)-1)])
		machine.orgObjectNameForKeyPrefix("xyz")
	}
	if scans := keyPrefixScans.Load() - before; scans != 0 {
		t.Fatalf("%d full passes over objects in 200 lookups, want 0", scans)
	}
}

// The index is shared by every org clone of one generation.
func TestKeyPrefixIndexSharedByClones(t *testing.T) {
	org, prefixes := frozenPrefixOrg(rand.New(rand.NewSource(8)), 50)
	first := New(nil)
	first.Org = &org
	first.orgObjectNameForKeyPrefix(prefixes[1])
	clone := org.Clone()
	second := New(nil)
	second.Org = &clone
	before := keyPrefixScans.Load()
	wantName, wantOK := referenceObjectNameForKeyPrefix(&clone, prefixes[2])
	gotName, gotOK := second.orgObjectNameForKeyPrefix(prefixes[2])
	if gotName != wantName || gotOK != wantOK {
		t.Fatalf("clone lookup got %q, %v; want %q, %v", gotName, gotOK, wantName, wantOK)
	}
	if scans := keyPrefixScans.Load() - before; scans != 0 {
		t.Fatalf("clone lookup scanned %d times, want 0", scans)
	}
}

// Unstamped orgs are mutable and never use the index.
func TestKeyPrefixLookupUnstampedOrgScans(t *testing.T) {
	org, prefixes := frozenPrefixOrg(rand.New(rand.NewSource(9)), 30)
	org.RuntimeSchemaStamp = ""
	machine := New(nil)
	machine.Org = &org
	name, _ := machine.orgObjectNameForKeyPrefix(prefixes[1])
	// A write to a definition is seen at once.
	state := org.Objects[name]
	state.Definition.KeyPrefix = "q9q"
	org.Objects[name] = state
	if got, ok := machine.orgObjectNameForKeyPrefix("q9q"); !ok || got != name {
		t.Fatalf("unstamped lookup after a prefix write = %q, %v; want %q", got, ok, name)
	}
}

// Every in-place schema writer clears the stamp. After it the lookup must
// follow the new definitions, not the index of the old generation.
func TestKeyPrefixIndexFollowsSchemaChanges(t *testing.T) {
	org, prefixes := frozenPrefixOrg(rand.New(rand.NewSource(10)), 40)
	machine := New(nil)
	machine.Org = &org
	machine.orgObjectNameForKeyPrefix(prefixes[1])

	// Changed prefix, stamp cleared and restored, as a metadata path does.
	name, _ := referenceObjectNameForKeyPrefix(&org, prefixes[1])
	state := org.Objects[name]
	state.Definition = state.Definition.Clone()
	state.Definition.KeyPrefix = "q9q"
	org.Objects[name] = state
	org.ClearRuntimeSchemaStamp()
	org.RuntimeSchemaStamp = "test-stamp-2"
	if got, ok := machine.orgObjectNameForKeyPrefix("q9q"); !ok || got != name {
		t.Fatalf("lookup after a prefix change = %q, %v; want %q", got, ok, name)
	}
	wantName, wantOK := referenceObjectNameForKeyPrefix(&org, prefixes[1])
	if got, ok := machine.orgObjectNameForKeyPrefix(prefixes[1]); got != wantName || ok != wantOK {
		t.Fatalf("lookup of the old prefix = %q, %v; want %q, %v", got, ok, wantName, wantOK)
	}

	// An added object under the same stamp (object count guard).
	org.Objects["Aaa_New__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Aaa_New__c", KeyPrefix: "q8q"}}
	if got, ok := machine.orgObjectNameForKeyPrefix("q8q"); !ok || got != "Aaa_New__c" {
		t.Fatalf("lookup after an added object = %q, %v", got, ok)
	}

}

// A hit whose definition changed without a stamp clear is re-checked against
// the object's current prefix.
func TestKeyPrefixIndexRechecksHit(t *testing.T) {
	org, prefixes := frozenPrefixOrg(rand.New(rand.NewSource(11)), 40)
	machine := New(nil)
	machine.Org = &org
	name, ok := machine.orgObjectNameForKeyPrefix(prefixes[1])
	if !ok {
		t.Fatal("no object holds the first prefix")
	}
	state := org.Objects[name]
	state.Definition.KeyPrefix = "q6q"
	org.Objects[name] = state
	wantName, wantOK := referenceObjectNameForKeyPrefix(&org, prefixes[1])
	if got, ok := machine.orgObjectNameForKeyPrefix(prefixes[1]); got != wantName || ok != wantOK {
		t.Fatalf("stale index hit: got %q, %v; want %q, %v", got, ok, wantName, wantOK)
	}
}

func BenchmarkKeyPrefixLookupFrozen(b *testing.B) {
	org := storage.NewOrgState()
	storage.EnsureStandardObjectSpread(&org)
	org.RuntimeSchemaStamp = fmt.Sprintf("bench-%d", 1)
	machine := New(nil)
	machine.Org = &org
	machine.orgObjectNameForKeyPrefix("003")
	b.ReportAllocs()
	for b.Loop() {
		machine.orgObjectNameForKeyPrefix("003")
	}
}

package storage

import (
	"strings"
	"testing"
)

// The exact catalog path must preserve the existing resolver for every spelling,
// including folded aliases and inputs that are not in the catalog.
func TestStandardObjectExactCatalogMemo(t *testing.T) {
	names := KnownStandardObjectNames()
	for _, name := range names {
		for _, spelling := range []string{name, strings.ToLower(name), strings.ToUpper(name), " " + name + " "} {
			want, wantOK := resolveKnownStandardObjectNameUncached(spelling)
			got, gotOK := ResolveKnownStandardObjectName(spelling)
			if got != want || gotOK != wantOK {
				t.Fatalf("resolve %q = %q, %v; want %q, %v", spelling, got, gotOK, want, wantOK)
			}
		}
	}
	for _, spelling := range []string{"", "   ", "NoSuchObject__c", "schema.account", "K", "\xff"} {
		want, wantOK := resolveKnownStandardObjectNameUncached(spelling)
		got, gotOK := ResolveKnownStandardObjectName(spelling)
		if got != want || gotOK != wantOK {
			t.Fatalf("resolve %q = %q, %v; want %q, %v", spelling, got, gotOK, want, wantOK)
		}
	}
	// Exact spellings whose folded winner is themselves remain available after
	// the bounded memo is cleared, including a second full catalog traversal.
	resetKnownStandardObjectMemo()
	before := knownStandardObjectResolves.Load()
	for _, name := range names {
		if name != "" && knownStandardObjectCache.canonicalByLC[standardObjectLookupKey(name)] == name {
			ResolveKnownStandardObjectName(name)
		}
	}
	if n := knownStandardObjectResolves.Load() - before; n != 0 {
		t.Fatalf("exact catalog traversal performed %d uncached resolutions, want 0", n)
	}
}

func TestStandardObjectExactCatalogMemoReset(t *testing.T) {
	resetKnownStandardObjectCacheForTest()
	t.Cleanup(resetKnownStandardObjectCacheForTest)
	names := KnownStandardObjectNames()
	if len(names) == 0 {
		t.Fatal("empty catalog")
	}
	name := names[0]
	names[0] = "ChangedByCaller"
	if got := KnownStandardObjectNames()[0]; got != name {
		t.Fatalf("catalog name changed to %q after caller mutation, want %q", got, name)
	}
	resetKnownStandardObjectMemo()
	got, ok := ResolveKnownStandardObjectName(name)
	want, wantOK := resolveKnownStandardObjectNameUncached(name)
	if got != want || ok != wantOK {
		t.Fatalf("lookup after mutation and memo reset = %q, %v; want %q, %v", got, ok, want, wantOK)
	}
	resetKnownStandardObjectCacheForTest()
	if knownStandardObjectExactNames.Load() != nil {
		t.Fatal("catalog reset retained exact names")
	}
	KnownStandardObjectNames()
	got, ok = ResolveKnownStandardObjectName(name)
	if got != want || ok != wantOK {
		t.Fatalf("lookup after catalog rebuild = %q, %v; want %q, %v", got, ok, want, wantOK)
	}
}

func BenchmarkStandardObjectExactCatalogSweep(b *testing.B) {
	names := KnownStandardObjectNames()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, name := range names {
			ResolveKnownStandardObjectName(name)
		}
	}
}

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

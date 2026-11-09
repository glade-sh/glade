package storage

import "testing"

// The VM resolves the same class-name spellings on every static call. Each
// answer must match the uncached resolution and a repeated lookup must not
// allocate.
func TestResolveKnownStandardObjectNameRepeatsAreFreeAndIdentical(t *testing.T) {
	names := []string{
		"Math", "math", "String", "System", "Account", "ACCOUNT", " Contact ", "contact",
		"Foo__c", "Bar__mdt", "Baz__e", "Qux__r", "GiftTransaction", "gifttransaction",
		"DocumentTemplate", "Opportunity", "", "   ", "NotAnObject", "Schema.Account",
	}
	for _, name := range names {
		wantName, wantOK := resolveKnownStandardObjectNameUncached(name)
		for i := 0; i < 3; i++ {
			gotName, gotOK := ResolveKnownStandardObjectName(name)
			if gotName != wantName || gotOK != wantOK {
				t.Fatalf("ResolveKnownStandardObjectName(%q) = %q,%v on call %d, uncached %q,%v", name, gotName, gotOK, i+1, wantName, wantOK)
			}
		}
	}
	// Every spelling has been seen once; a thousand more lookups must not
	// resolve again (fails before the memo: one resolution per call).
	before := knownStandardObjectResolves.Load()
	for i := 0; i < 1000; i++ {
		for _, name := range names {
			ResolveKnownStandardObjectName(name)
		}
	}
	if resolves := knownStandardObjectResolves.Load() - before; resolves != 0 {
		t.Fatalf("%d uncached resolutions for repeated spellings, want 0", resolves)
	}
	allocs := testing.AllocsPerRun(50, func() {
		for _, name := range names {
			ResolveKnownStandardObjectName(name)
		}
	})
	if allocs != 0 {
		t.Fatalf("repeated ResolveKnownStandardObjectName allocated %.1f times per pass, want 0", allocs)
	}
}

// Past the memo limit a lookup still answers correctly and the memo stops growing.
func TestResolveKnownStandardObjectNameMemoIsBounded(t *testing.T) {
	resetKnownStandardObjectCacheForTest()
	t.Cleanup(resetKnownStandardObjectCacheForTest)
	for i := 0; i < knownStandardObjectMemoLimit+64; i++ {
		name := "NoSuchType" + string(rune('A'+i%26)) + string(rune('A'+(i/26)%26)) + string(rune('A'+(i/676)%26))
		if _, ok := ResolveKnownStandardObjectName(name); ok {
			t.Fatalf("%q resolved as a standard object", name)
		}
	}
	knownStandardObjectMemo.mu.Lock()
	count := knownStandardObjectMemo.count
	knownStandardObjectMemo.mu.Unlock()
	if count > knownStandardObjectMemoLimit {
		t.Fatalf("memo holds %d entries, limit %d", count, knownStandardObjectMemoLimit)
	}
	if name, ok := ResolveKnownStandardObjectName("account"); !ok || name != "Account" {
		t.Fatalf("Account resolution after the memo filled = %q,%v", name, ok)
	}
	// A spelling asked after the memo filled is retained for the next ask: an
	// org build fills the memo once with every catalog name before any Apex
	// runs, and the names Apex then repeats must still be served from it.
	before := knownStandardObjectResolves.Load()
	ResolveKnownStandardObjectName("account")
	ResolveKnownStandardObjectName("Math")
	ResolveKnownStandardObjectName("Math")
	if resolves := knownStandardObjectResolves.Load() - before; resolves != 1 {
		t.Fatalf("%d uncached resolutions after the memo filled, want 1 (the first Math)", resolves)
	}
}

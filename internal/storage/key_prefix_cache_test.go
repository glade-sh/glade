package storage

import "testing"

func TestCapturedKeyPrefixLookupAvoidsRepeatedCatalogWork(t *testing.T) {
	const name = "CareProgram"
	native, known := StandardObjectDescribeKeyPrefix(name)
	if !known {
		t.Fatal("missing captured describe")
	}
	want := ""
	if native != nil {
		want = *native
		if _, valid := keyPrefixBitsetIndex(want); !valid {
			want = ""
		}
	}
	if got := capturedStandardKeyPrefix(name); got != want {
		t.Fatalf("prefix %q; want %q", got, want)
	}
	allocs := testing.AllocsPerRun(20, func() {
		if got := capturedStandardKeyPrefix(name); got != want {
			t.Fatalf("prefix %q; want %q", got, want)
		}
	})
	t.Logf("warmed captured prefix lookup: %.0f allocations", allocs)
	if allocs != 0 {
		t.Fatalf("warmed prefix lookup allocates %.0f; want zero", allocs)
	}
	for _, alias := range []string{"careprogram", " CAREPROGRAM "} {
		if got := capturedStandardKeyPrefix(alias); got != want {
			t.Fatalf("alias %q: %q; want %q", alias, got, want)
		}
	}
	if got := capturedStandardKeyPrefix("OwnedUnrecognized__c"); got != "" {
		t.Fatal(got)
	}
}

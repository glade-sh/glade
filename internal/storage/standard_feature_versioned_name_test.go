package storage

import (
	"strings"
	"testing"
)

// Local performance contract: the versioned-object lookup folds case like
// strings.ToLower and does not allocate for ASCII names.
func TestStandardFeatureVersionedObjectNameFolds(t *testing.T) {
	loadStandardFeatureDescribes()
	names := standardFeatureDescribeCache.versionedNames
	if len(names) == 0 {
		t.Fatal("no versioned objects")
	}
	for key, want := range names {
		for _, input := range []string{key, strings.ToUpper(key), want} {
			if got, ok := standardFeatureVersionedObjectName(input); !ok || got != want {
				t.Fatalf("%q: got %q %t, want %q", input, got, ok, want)
			}
		}
	}
	for _, input := range []string{"Account", "", "Ünknown__c", strings.Repeat("A", 80)} {
		got, ok := standardFeatureVersionedObjectName(input)
		want, wantOK := names[strings.ToLower(input)]
		if got != want || ok != wantOK {
			t.Fatalf("%q: got %q %t, want %q %t", input, got, ok, want, wantOK)
		}
	}
	if allocs := testing.AllocsPerRun(100, func() { standardFeatureVersionedObjectName("Account") }); allocs != 0 {
		t.Fatalf("lookup allocates %v times", allocs)
	}
}

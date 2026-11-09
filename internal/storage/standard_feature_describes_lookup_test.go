package storage

import (
	"strings"
	"testing"
)

// Every catalog name resolution consults the feature describe bundle first
// (standardDescribeCatalogCanonicalName, standardObjectCatalogEntryForName,
// standardDescribeFieldForName), so the miss path has to stay cheap.
func TestStandardFeatureDescribeNameResolvesCaseInsensitively(t *testing.T) {
	for _, name := range []string{"GiftTransaction", "gifttransaction", " GIFTTRANSACTION ", "DocumentTemplate"} {
		got, ok := standardFeatureDescribeName(name)
		if !ok {
			t.Fatalf("standardFeatureDescribeName(%q) missed", name)
		}
		if _, listed := loadStandardFeatureDescribes().Objects[got]; !listed || !strings.EqualFold(got, strings.TrimSpace(name)) {
			t.Fatalf("standardFeatureDescribeName(%q) = %q, want the canonical bundle name", name, got)
		}
	}
	for _, name := range []string{"", "Account", "GiftTransactionShare", "Gift"} {
		if got, ok := standardFeatureDescribeName(name); ok {
			t.Fatalf("standardFeatureDescribeName(%q) = %q, want a miss", name, got)
		}
	}
	if canonical := standardFeatureDescribeCache.canonical; len(canonical) != len(standardFeatureDescribeCache.names) {
		t.Fatalf("canonical names = %d, want %d", len(canonical), len(standardFeatureDescribeCache.names))
	}
}

func TestStandardFeatureDescribeNameWarmMissAllocations(t *testing.T) {
	for _, name := range []string{"Account", "OpportunityLineItem", "Probe_Item__c"} {
		lookup := func() { standardFeatureDescribeName(name) }
		lookup()
		if got := testing.AllocsPerRun(100, lookup); got != 0 {
			t.Fatalf("warm miss allocations for %q = %.0f, want 0", name, got)
		}
	}
}

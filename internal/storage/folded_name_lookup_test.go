package storage

import (
	"strings"
	"testing"
)

func TestFoldedNameLookupPreservesLowercaseKeys(t *testing.T) {
	for _, name := range []string{"", " Account ", "SCHEMA.Account", "\u2003MixedName__c\u00a0", "Straße", "Σςσ", "İIıi", "KKk", "ſSs", "A\xffZ"} {
		for range 2 {
			if got, want := standardObjectLookupKey(name), strings.ToLower(strings.TrimSpace(name)); got != want {
				t.Errorf("lookup key(%q) = %q, want %q", name, got, want)
			}
		}
	}
	for _, name := range []string{"Account", "ACCOUNT", " account ", "\u2003Account\u00a0"} {
		entry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, name)
		if !ok || entry.Name != "Account" {
			t.Fatalf("catalog lookup(%q) = %q, %v; want Account, true", name, entry.Name, ok)
		}
	}
}

func TestFoldedNameLookupWarmAllocations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		lookup func()
	}{
		{"standard key", func() { standardObjectLookupKey("OpportunityLineItem") }},
		{"catalog index", func() { lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, "OpportunityLineItem") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.lookup()
			if got := testing.AllocsPerRun(100, tc.lookup); got != 0 {
				t.Fatalf("warm lookup allocations = %.0f, want 0", got)
			}
		})
	}
}

var foldedNameKeyResult string
var foldedNameCatalogResult standardDescribeCatalogV2IndexEntry

func BenchmarkFoldedNameLookup(b *testing.B) {
	b.Run("StandardKey", func(b *testing.B) {
		standardObjectLookupKey("OpportunityLineItem")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			foldedNameKeyResult = standardObjectLookupKey("OpportunityLineItem")
		}
	})
	b.Run("CatalogIndex", func(b *testing.B) {
		lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, "OpportunityLineItem")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			foldedNameCatalogResult, _ = lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, "OpportunityLineItem")
		}
	})
}

func TestFoldedNameFieldFeatureRoutes(t *testing.T) {
	for _, features := range []string{"", "PersonAccounts"} {
		definition := ObjectDefinition{APIName: "Account", Metadata: map[string]string{standardFieldsOverlayMarker: features}}
		for _, name := range []string{"Name", "NAME", "nAmE"} {
			if flag, known := StandardFieldNameFlag(definition, name); !flag || !known {
				t.Fatalf("%s/%s: %v %v", features, name, flag, known)
			}
		}
		if flag, known := StandardFieldNameFlag(definition, "Id"); flag || !known {
			t.Fatalf("%s/Id: %v %v", features, flag, known)
		}
		if _, known := StandardFieldNameFlag(definition, "Missing"); known {
			t.Fatal("unknown field became known")
		}
	}
}

func TestFoldedNameFieldWarmAllocationRoute(t *testing.T) {
	lookup := func() { StandardFieldNameFlag(ObjectDefinition{APIName: "aCcOuNt"}, "nAmE") }
	lookup()
	if got := testing.AllocsPerRun(100, lookup); got != 0 {
		t.Fatalf("warm name-field lookup allocations = %.0f, want 0", got)
	}
}

func TestFoldedNameRawCatalogWarmAllocationRoute(t *testing.T) {
	lookup := func() {
		if _, ok, err := lookupStandardDescribeCatalogV2("Account"); !ok || err != nil {
			t.Fatalf("raw catalog lookup: found=%v err=%v", ok, err)
		}
	}
	lookup()
	if got := testing.AllocsPerRun(100, lookup); got != 0 {
		t.Fatalf("warm raw catalog allocations = %.0f, want 0", got)
	}
}

package storage

import (
	"sort"
	"strings"
	"testing"
)

func legacyStandardDescribeIndexLookup(index []standardDescribeCatalogV2IndexEntry, objectName string) (standardDescribeCatalogV2IndexEntry, bool) {
	key := strings.ToLower(strings.TrimSpace(objectName))
	if key == "" {
		return standardDescribeCatalogV2IndexEntry{}, false
	}
	position := sort.Search(len(index), func(i int) bool { return strings.ToLower(index[i].Name) >= key })
	if position == len(index) || !strings.EqualFold(index[position].Name, key) {
		return standardDescribeCatalogV2IndexEntry{}, false
	}
	return index[position], true
}

func TestStandardDescribeNameComparisonMatchesLowercase(t *testing.T) {
	names := []string{"", "A", "a", "Account", "AccountHistory", "Account_", "Account0", "[", "_", "z", "\u212A", "\u017F", "\u0130", "\u03A3", "\u03C2", "\u00C9", "A\u212A", "a\u0130z", "\xff", "A\xc0\xaf"}
	for _, name := range names {
		for _, query := range names {
			key := strings.ToLower(query)
			if got, want := standardDescribeNameAtLeast(name, key), strings.ToLower(name) >= key; got != want {
				t.Fatalf("compare(%q, %q) = %v, want %v", name, key, got, want)
			}
		}
	}
	// Cover byte ordering at every possible first mismatch, including invalid
	// UTF-8 and suffixes that change byte length during Unicode lowercasing.
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			name := "Prefix" + string([]byte{byte(a)}) + "\u212A"
			key := strings.ToLower("prefix" + string([]byte{byte(b)}) + "k")
			if got, want := standardDescribeNameAtLeast(name, key), strings.ToLower(name) >= key; got != want {
				t.Fatalf("compare(%q, %q) = %v, want %v", name, key, got, want)
			}
		}
	}
}

func TestStandardDescribeIndexLookupMatchesLegacy(t *testing.T) {
	unicodeIndex := []standardDescribeCatalogV2IndexEntry{{Name: "A"}, {Name: "A\u212A"}, {Name: "\u0130"}, {Name: "\u212A"}, {Name: "\u017F"}, {Name: "\u03A3"}, {Name: "\xff"}}
	sort.Slice(unicodeIndex, func(i, j int) bool {
		return strings.ToLower(unicodeIndex[i].Name) < strings.ToLower(unicodeIndex[j].Name)
	})
	for _, index := range [][]standardDescribeCatalogV2IndexEntry{nil, standardDescribeCatalogV2Index, standardDescribeChildRelationshipsV2Index, unicodeIndex} {
		queries := []string{"", " ", "!", "Account!", "Account_", "zzzzzzzz", "NotAnObject__c", "k", "s", "i", "\u03C2", "\xff"}
		for _, entry := range index {
			queries = append(queries, entry.Name, strings.ToLower(entry.Name), strings.ToUpper(entry.Name), " \t"+entry.Name+"\n", entry.Name+"!")
		}
		for _, query := range queries {
			got, ok := lookupStandardDescribeCatalogV2Index(index, query)
			want, wantOK := legacyStandardDescribeIndexLookup(index, query)
			if got != want || ok != wantOK {
				t.Fatalf("lookup(%q) = (%+v, %v), want (%+v, %v)", query, got, ok, want, wantOK)
			}
		}
	}
}

func BenchmarkStandardDescribeIndexLookup(b *testing.B) {
	queries := []string{"Account", "account", "CAREPROGRAM", "OpportunityLineItem", "notAnObject", "  User  "}
	for _, impl := range []struct {
		name   string
		lookup func([]standardDescribeCatalogV2IndexEntry, string) (standardDescribeCatalogV2IndexEntry, bool)
	}{{"legacy", legacyStandardDescribeIndexLookup}, {"foldedComparison", lookupStandardDescribeCatalogV2Index}} {
		b.Run(impl.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				impl.lookup(standardDescribeCatalogV2Index, queries[i%len(queries)])
			}
		})
	}
}

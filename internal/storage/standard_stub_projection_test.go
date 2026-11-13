package storage

import (
	"reflect"
	"testing"
)

func TestStandardStubProjectionFacetsMatchFull(t *testing.T) {
	for _, entry := range standardDescribeCatalogV2Index {
		full, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		narrow, err := decodeStandardDescribeCatalogV2Narrow(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		if want := standardDescribeNarrowFromDescribe(full).fields; !reflect.DeepEqual(narrow.fields, want) {
			t.Fatalf("%s: projected facets differ", entry.Name)
		}
	}
}

func TestStandardStubProjectionAvoidsFullDecode(t *testing.T) {
	var names []string
	for _, entry := range standardDescribeCatalogV2Index {
		if !standardSObjectStubHasFields(entry.Name) {
			continue
		}
		if _, ok := standardObjectCatalogEntryForName(entry.Name); !ok {
			names = append(names, entry.Name)
		}
	}
	standardDescribeCatalogV2RawProductionCache.release()
	before := standardDescribeFullDecodes.Load()
	for _, name := range names {
		definition := ObjectDefinition{APIName: name, Fields: make(map[string]Field)}
		mergeStandardObjectDefinition(&definition, nil)
		mergeStandardSObjectStubFields(&definition, nil)
		standardStubReferenceFieldsFromV2Describe(name)
		standardStubTimeCatalogFields(name)
		standardStubCurrencyCatalogFields(name)
	}
	if len(names) == 0 {
		t.Fatal("no stub-only objects")
	}
	if got := standardDescribeFullDecodes.Load() - before; got != 0 {
		t.Fatalf("%d stub-only objects caused %d full decodes", len(names), got)
	}
	t.Logf("%d stub-only objects avoided full decode", len(names))
}

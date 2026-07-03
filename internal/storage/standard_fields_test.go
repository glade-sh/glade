package storage

import (
	"sync"
	"testing"
)

func TestKnownStandardObjectNamesDoesNotHydrateDescribeCatalog(t *testing.T) {
	resetKnownStandardObjectCacheForTest()
	defer resetKnownStandardObjectCacheForTest()

	names := KnownStandardObjectNames()
	if len(names) == 0 {
		t.Fatalf("KnownStandardObjectNames returned no names")
	}
	if standardObjectCatalogLookupCache.describeByLC != nil {
		t.Fatalf("KnownStandardObjectNames hydrated %d catalog entries", len(standardObjectCatalogLookupCache.describeByLC))
	}
	if !stringSliceContains(names, "CareProgram") {
		t.Fatalf("KnownStandardObjectNames missing describe-only object CareProgram")
	}
}

func TestUnknownStandardObjectNameCheckDoesNotHydrateDescribeCatalog(t *testing.T) {
	resetKnownStandardObjectCacheForTest()
	defer resetKnownStandardObjectCacheForTest()

	if IsKnownStandardObject("Util") {
		t.Fatalf("Util should not be a known standard object")
	}
	if standardObjectCatalogLookupCache.describeByLC != nil {
		t.Fatalf("unknown name check hydrated %d catalog entries", len(standardObjectCatalogLookupCache.describeByLC))
	}
}

func TestStandardDescribeCatalogObjectNamesStayInSync(t *testing.T) {
	catalog := loadEmbeddedStandardDescribeCatalog()
	for name := range catalog {
		if !stringSliceContains(standardDescribeCatalogObjectNames, name) {
			t.Fatalf("standardDescribeCatalogObjectNames missing %s", name)
		}
	}
}

func resetKnownStandardObjectCacheForTest() {
	knownStandardObjectCache = struct {
		once          sync.Once
		names         []string
		canonicalByLC map[string]string
		catalogByLC   map[string]standardObjectCatalogEntry
	}{}
	standardObjectCatalogLookupCache = struct {
		generatedOnce    sync.Once
		generatedByLC    map[string]standardObjectCatalogEntry
		describeNameOnce sync.Once
		describeNameByLC map[string]string
		describeOnce     sync.Once
		describeByLC     map[string]standardObjectCatalogEntry
	}{}
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

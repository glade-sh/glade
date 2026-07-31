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

func TestStandardObjectDefinitionLeadEmailIDLookup(t *testing.T) {
	def, ok := StandardObjectDefinition("Lead")
	if !ok {
		t.Fatal("StandardObjectDefinition Lead not found")
	}
	email, ok := def.Fields["Email"]
	if !ok {
		t.Fatal("Lead.Email field not found")
	}
	if !email.IDLookup {
		t.Fatal("Lead.Email IDLookup = false, want true")
	}
}

func TestStandardObjectDefinitionAccountNameIDLookupFalse(t *testing.T) {
	def, ok := StandardObjectDefinition("Account")
	if !ok {
		t.Fatal("StandardObjectDefinition Account not found")
	}
	name, ok := def.Fields["Name"]
	if !ok {
		t.Fatal("Account.Name field not found")
	}
	if name.IDLookup {
		t.Fatal("Account.Name IDLookup = true, want false")
	}
}

func TestStandardObjectDefinitionNoFeatureGatedFieldsFromEnrichment(t *testing.T) {
	def, ok := StandardObjectDefinition("Account")
	if !ok {
		t.Fatal("StandardObjectDefinition Account not found")
	}
	if _, ok := def.Fields["PersonEmail"]; ok {
		t.Fatal("Account should not have PersonEmail without PersonAccounts feature")
	}
	if _, ok := def.Fields["FirstName"]; ok {
		t.Fatal("Account should not have FirstName without PersonAccounts feature")
	}
}

func TestV2DescribeIDLookupDecodeRoundTrip(t *testing.T) {
	describe, ok, err := lookupStandardDescribeCatalogV2("Lead")
	if err != nil || !ok {
		t.Fatalf("lookup Lead v2: ok=%v err=%v", ok, err)
	}
	var emailIDLookup bool
	for _, field := range describe.Fields {
		if field.Name == "Email" {
			emailIDLookup = field.IDLookup
			break
		}
	}
	if !emailIDLookup {
		t.Fatal("v2 describe Lead.Email idLookup not decoded")
	}
}

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

func TestUserRoleOpportunityAccessDefaultsToEdit(t *testing.T) {
	definition, ok := StandardObjectDefinition("UserRole")
	if !ok {
		t.Fatal("UserRole standard definition not found")
	}
	field, ok := definition.Fields["OpportunityAccessForAccountOwner"]
	if !ok {
		t.Fatal("UserRole.OpportunityAccessForAccountOwner not found")
	}
	if !field.Required {
		t.Fatal("UserRole.OpportunityAccessForAccountOwner must remain required")
	}
	if field.DefaultValue != "Edit" {
		t.Fatalf("UserRole.OpportunityAccessForAccountOwner default = %q, want Edit", field.DefaultValue)
	}
	value, ok := DefaultValueForField(field)
	if !ok || value.String != "Edit" {
		t.Fatalf("default value = %#v, %v; want Edit, true", value, ok)
	}
}

func TestKnownStandardObjectNamesIncludeV2DescribeIndex(t *testing.T) {
	resetKnownStandardObjectCacheForTest()
	defer resetKnownStandardObjectCacheForTest()

	names := KnownStandardObjectNames()
	for _, entry := range standardDescribeCatalogV2Index {
		if !stringSliceContains(names, entry.Name) {
			t.Fatalf("KnownStandardObjectNames missing V2 describe object %s", entry.Name)
		}
	}
	for _, name := range []string{
		"AIMetric",
		"ActionPlanItemDependency",
		"ActionPlnTmplItmDependency",
		"DataAssetSemanticGraphEdge",
		"FinanceBalanceSnapshot",
		"FinanceTransaction",
	} {
		if !stringSliceContains(names, name) {
			t.Fatalf("KnownStandardObjectNames missing Webhook2Flow target %s", name)
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

func TestMergeStandardFieldsCaseVariantOrderIsDeterministic(t *testing.T) {
	fields := map[string]Field{
		"UserName": {APIName: "UserName", Label: "User Name"},
		"Username": {APIName: "Username", Label: "Username"},
	}
	for i := 0; i < 10; i++ {
		definition := ObjectDefinition{Fields: make(map[string]Field)}
		mergeStandardFields(&definition, fields)
		if _, ok := definition.Fields["UserName"]; !ok {
			t.Fatalf("merge retained %v on run %d, want UserName", definition.Fields, i+1)
		}
		if _, ok := definition.Fields["Username"]; ok {
			t.Fatalf("merge retained case-variant Username on run %d", i+1)
		}
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

func TestEnsureStandardObjectFieldsRefreshesStalePicklistMetadata(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "Account",
		Fields: map[string]Field{
			"Rating": {APIName: "Rating", Type: FieldPicklist},
		},
		Metadata: map[string]string{standardFieldsOverlayMarker: ""},
	}

	EnsureStandardObjectFields(&definition)

	rating := definition.Fields["Rating"]
	if len(rating.PicklistValues) != 3 {
		t.Fatalf("Account.Rating picklist values = %#v, want Hot/Warm/Cold", rating.PicklistValues)
	}
	for i, want := range []string{"Hot", "Warm", "Cold"} {
		if rating.PicklistValues[i].Value != want {
			t.Fatalf("Account.Rating picklist value %d = %#v, want %q", i, rating.PicklistValues[i], want)
		}
	}
}

func TestEnsureStandardObjectFieldsRefreshesShallowMetadataWhenOverlayMarked(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "Account",
		Fields: map[string]Field{
			"Name":  {APIName: "Name", Type: FieldString, DisplayType: "ANY"},
			"Phone": {APIName: "Phone", Type: FieldString, DisplayType: "ANY"},
		},
		Metadata: map[string]string{standardFieldsOverlayMarker: ""},
	}

	EnsureStandardObjectFields(&definition)

	if got := definition.Fields["Name"].DisplayType; got != "STRING" {
		t.Fatalf("Account.Name display type = %q, want STRING", got)
	}
	if got := definition.Fields["Phone"].DisplayType; got != "PHONE" {
		t.Fatalf("Account.Phone display type = %q, want PHONE", got)
	}
}

func TestEnsureStandardObjectFieldsCorrectsQuickTextChannelMultiPicklist(t *testing.T) {
	definition := ObjectDefinition{APIName: "QuickText"}

	EnsureStandardObjectFields(&definition)

	channel, ok := definition.Fields["Channel"]
	if !ok {
		t.Fatal("QuickText.Channel missing")
	}
	if channel.Type != FieldMultiPicklist || channel.DisplayType != "MULTIPICKLIST" {
		t.Fatalf("QuickText.Channel metadata = %#v, want MULTIPICKLIST", channel)
	}
	name, ok := definition.Fields["Name"]
	if !ok || name.Type != FieldString || name.DisplayType != "STRING" {
		t.Fatalf("QuickText.Name metadata = %#v, %v; unrelated field changed", name, ok)
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

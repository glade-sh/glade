package storage

import (
	"reflect"
	"strings"
	"sync"
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

func legacyStubWriteFlags(definition *ObjectDefinition) {
	describe, ok, err := lookupStandardDescribeCatalogV2(definition.APIName)
	if err != nil || !ok {
		return
	}
	stubFields, _ := standardSObjectStubFieldsFor(definition.APIName)
	for _, captured := range describe.Fields {
		name, exists := ResolveFieldName(*definition, "", captured.Name)
		if !exists {
			continue
		}
		field := definition.Fields[name]
		if stub, exists := stubFields[captured.Name]; exists && (captured.Calculated || stub.Type == FieldDecimal && strings.EqualFold(stub.DisplayType, "DOUBLE") && strings.EqualFold(captured.Type, "currency")) &&
			field.Type == stub.Type && field.DisplayType == stub.DisplayType && field.Formula == "" {
			if captured.Calculated {
				field.Type = FieldCalculated
			}
			field.DisplayType = describeDisplayType(captured.Type)
		}
		if strings.EqualFold(captured.Type, "time") && standardStubTimeReference(field) {
			field.Type = describeFieldType(captured)
			field.DisplayType = describeDisplayType(captured.Type)
			field.ReferenceTo = append([]string(nil), captured.ReferenceTo...)
			field.RelationshipName = captured.RelationshipName
			field.Aggregatable = cloneBoolFlag(captured.Aggregatable)
		}
		if standardStubScaleFromDescribe(definition.APIName) && field.Precision == 0 && field.Scale == 0 {
			field.Precision, field.Scale, field.ScaleSpecified = captured.Precision, captured.Scale, true
		}
		if field.Createable == nil {
			field.Createable = cloneBoolFlag(captured.Createable)
		}
		if field.Updateable == nil {
			field.Updateable = cloneBoolFlag(captured.Updateable)
		}
		definition.Fields[name] = field
	}
}

func TestStandardStubWriteFlagsProjectionMatchesFull(t *testing.T) {
	for _, entry := range standardDescribeCatalogV2Index {
		fields, ok := standardSObjectStubFieldsFor(entry.Name)
		if !ok {
			continue
		}
		for _, existing := range []bool{false, true} {
			definition := ObjectDefinition{APIName: entry.Name, Fields: make(map[string]Field)}
			for name, field := range fields {
				if existing {
					field.Createable = BoolFlag(false)
					field.Updateable = BoolFlag(true)
					field.Precision = 2
					field.Scale = 1
				}
				definition.Fields[name] = field
			}
			want, got := definition.Clone(), definition.Clone()
			enrichStandardStubWriteFlagsFromV2Describe(&got)
			legacyStubWriteFlags(&want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s existing=%v: projected write flags differ", entry.Name, existing)
			}
		}
	}
}

func TestStandardStubMetadataProjectionMatchesFull(t *testing.T) {
	for _, entry := range standardDescribeCatalogV2Index {
		full, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		narrow, err := decodeStandardDescribeCatalogV2Narrow(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(narrow.mergeable, full.Mergeable) {
			t.Fatalf("%s: mergeable projection differs", entry.Name)
		}
		for _, configured := range []bool{false, true} {
			want := ObjectDefinition{APIName: entry.Name}
			if configured {
				want.Metadata = map[string]string{"mergeable": "custom"}
			}
			got := want.Clone()
			enrichStandardDescribeMetadata(&want, full)
			enrichStandardDescribeMetadataFromV2(&got)
			if !reflect.DeepEqual(got.Metadata, want.Metadata) {
				t.Fatalf("%s configured=%v: metadata got %v, want %v", entry.Name, configured, got.Metadata, want.Metadata)
			}
		}
	}
}

func TestStandardStubEnrichmentProjectionMatchesFull(t *testing.T) {
	for _, entry := range standardDescribeCatalogV2Index {
		full, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		projected, err := decodeStandardDescribeCatalogV2Fields(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(projected.Fields, full.Fields) || !reflect.DeepEqual(projected.Mergeable, full.Mergeable) {
			t.Fatalf("%s: enrichment fields differ", entry.Name)
		}
	}
}

func TestStandardStubEnrichmentAvoidsFullDecode(t *testing.T) {
	names := standardStubDescribeObjectNames(t)
	standardDescribeCatalogV2RawProductionCache.release()
	before := standardDescribeFullDecodes.Load()
	for _, name := range names {
		fields, _ := standardSObjectStubFieldsFor(name)
		definition := (ObjectDefinition{APIName: name, Fields: fields}).Clone()
		enrichStandardFieldsFromV2Describe(&definition)
		enrichRequiredStandardStubFieldsFromV2Describe(&definition)
	}
	if got := standardDescribeFullDecodes.Load() - before; got != 0 {
		t.Fatalf("%d stub enrichments caused %d full decodes", len(names), got)
	}
	t.Logf("%d stub enrichments avoided full decode", len(names))
}

func TestStandardStubEnrichmentConcurrentLookups(t *testing.T) {
	standardDescribeCatalogV2RawProductionCache.release()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if i%2 == 0 {
					if _, ok, err := lookupStandardDescribeFieldsV2("AccountBrand"); err != nil || !ok {
						t.Errorf("fields lookup: %v", err)
					}
				} else {
					if _, ok, err := lookupStandardDescribeCatalogV2("AccountBrand"); err != nil || !ok {
						t.Errorf("full lookup: %v", err)
					}
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestStandardFieldReadersAvoidFullDescribe(t *testing.T) {
	standardDescribeCatalogV2RawProductionCache.release()
	before := standardDescribeFullDecodes.Load()
	for _, name := range []string{"Account", "Contact", "Lead", "Order"} {
		standardNativeIDLabel(name)
	}
	for _, name := range []string{"PermissionSet", "ObjectPermissions"} {
		definition := ObjectDefinition{APIName: name, Fields: make(map[string]Field)}
		permissionSetCheckboxSchemaNeedsRepair(definition)
		applyPermissionSetCheckboxSchema(&definition)
		objectPermissionCheckboxSchemaNeedsRepair(definition)
		applyObjectPermissionCheckboxFacets(&definition)
	}
	if got := standardDescribeFullDecodes.Load() - before; got != 0 {
		t.Fatalf("field-only readers caused %d full decodes", got)
	}
}

func TestStandardFieldFacetLookupAvoidsFullDescribe(t *testing.T) {
	standardDescribeCatalogV2RawProductionCache.release()
	before := standardDescribeFullDecodes.Load()
	for _, entry := range standardDescribeCatalogV2Index {
		standardDescribeFieldForName(entry.Name, "Name")
	}
	if got := standardDescribeFullDecodes.Load() - before; got != 0 {
		t.Fatalf("field facet lookups caused %d full decodes", got)
	}
}

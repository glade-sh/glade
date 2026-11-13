package storage

import (
	"reflect"
	"strings"
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
			legacyStubWriteFlags(&want)
			enrichStandardStubWriteFlagsFromV2Describe(&got)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s existing=%v: projected write flags differ", entry.Name, existing)
			}
		}
	}
}

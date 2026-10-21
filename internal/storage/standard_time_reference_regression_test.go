package storage

import (
	"slices"
	"strings"
	"testing"
)

func TestStandardTimeFieldsRetainCatalogScalarShapeAcrossStubMerges(t *testing.T) {
	// Reuse the owned describe catalog; this does not add an API/context claim.
	captured, ok, err := lookupStandardDescribeCatalogV2("BusinessHours")
	if err != nil || !ok {
		t.Fatalf("BusinessHours catalog lookup: found=%v err=%v", ok, err)
	}
	definition, ok := StandardObjectDefinition("BusinessHours")
	if !ok {
		t.Fatal("missing BusinessHours definition")
	}
	assertScalarFields := func(stage string) {
		t.Helper()
		count := 0
		for _, observed := range captured.Fields {
			if !strings.EqualFold(observed.Type, "time") {
				continue
			}
			count++
			field, exists := definition.Fields[observed.Name]
			if !exists || field.Type != describeFieldType(observed) || field.DisplayType != describeDisplayType(observed.Type) ||
				!slices.Equal(field.ReferenceTo, observed.ReferenceTo) ||
				field.RelationshipName != observed.RelationshipName {
				t.Errorf("%s %s: got type=%s display=%s refs=%v relationship=%q; catalog type=%s refs=%v relationship=%q", stage, observed.Name,
					field.Type, field.DisplayType, field.ReferenceTo, field.RelationshipName, observed.Type, observed.ReferenceTo, observed.RelationshipName)
			}
		}
		if count != 14 {
			t.Fatalf("catalog Time field count = %d, want 14", count)
		}
	}
	assertScalarFields("fresh")
	for repeat := 0; repeat < 2; repeat++ {
		mergeStandardSObjectStubFields(&definition, nil)
		assertScalarFields("repeated stub merge")
		EnsureStandardObjectFields(&definition)
		assertScalarFields("repeated ensure")
	}
	if field := definition.Fields["CreatedById"]; field.Type != FieldReference || !slices.Equal(field.ReferenceTo, []string{"User"}) {
		t.Fatalf("real reference changed: %#v", field)
	}
}

func TestStandardTimeCatalogRepairPreservesExplicitWriteFlags(t *testing.T) {
	definition := ObjectDefinition{APIName: "BusinessHours", Fields: map[string]Field{
		"MondayStartTime": {
			APIName: "MondayStartTime", Type: FieldTime, DisplayType: "TIME", ReferenceTo: []string{"Time"},
			Createable: BoolFlag(false), Updateable: BoolFlag(true),
		},
	}}
	EnsureStandardObjectFields(&definition)
	field := definition.Fields["MondayStartTime"]
	if field.Type != FieldTime || field.DisplayType != "TIME" || len(field.ReferenceTo) != 0 || field.RelationshipName != "" {
		t.Fatalf("stale scalar reference survived catalog repair: %#v", field)
	}
	if field.Createable == nil || *field.Createable || field.Updateable == nil || !*field.Updateable {
		t.Fatalf("explicit write flags changed: create=%v update=%v", field.Createable, field.Updateable)
	}
}

func TestStandardTimeReferenceRepairDoesNotChangeUncapturedFields(t *testing.T) {
	// This is a preservation boundary, not a claim about native reference shape.
	definition := ObjectDefinition{APIName: "BusinessHours", Fields: map[string]Field{
		"NativeShapeControl__c": {
			APIName: "NativeShapeControl__c", Type: FieldTime, DisplayType: "TIME", ReferenceTo: []string{"Time"},
		},
		"CreatedById": {
			APIName: "CreatedById", Type: FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy",
		},
	}}
	enrichStandardStubWriteFlagsFromV2Describe(&definition)
	if field := definition.Fields["NativeShapeControl__c"]; field.Type != FieldTime || field.DisplayType != "TIME" || !slices.Equal(field.ReferenceTo, []string{"Time"}) {
		t.Fatalf("uncaptured field changed: %#v", field)
	}
	if field := definition.Fields["CreatedById"]; field.Type != FieldReference || !slices.Equal(field.ReferenceTo, []string{"User"}) || field.RelationshipName != "CreatedBy" {
		t.Fatalf("real reference changed: %#v", field)
	}
}

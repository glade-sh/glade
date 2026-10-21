package storage

import (
	"slices"
	"strings"
	"testing"
)

func TestStandardStubReferenceProjectionKeepsOnlyAvailableTargetShapes(t *testing.T) {
	previous := standardDescribeCatalogV2RawProductionCache
	standardDescribeCatalogV2RawProductionCache = newStandardDescribeCatalogV2RawCache(func(entry standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error) {
		return standardDescribeObject{Name: entry.Name, Fields: []standardDescribeField{
			{Name: "SupportedId", Type: "reference", ReferenceTo: []string{"Account", "Contact"}, RelationshipName: "Supported", Nillable: BoolFlag(false), Createable: BoolFlag(true), DefaultedOnCreate: BoolFlag(false)},
			{Name: "UnavailableId", Type: "reference", ReferenceTo: []string{"UnavailableOptionalRecord"}},
			{Name: "MixedId", Type: "reference", ReferenceTo: []string{"Account", "UnavailableOptionalRecord"}},
			{Name: "UnspecifiedId", Type: "reference"},
			{Name: "HiddenId", Type: "reference", ReferenceTo: []string{"Account"}, DeprecatedAndHidden: BoolFlag(true)},
		}}, nil
	})
	t.Cleanup(func() { standardDescribeCatalogV2RawProductionCache = previous })
	definition := ObjectDefinition{APIName: "AssociatedLocation", Fields: map[string]Field{
		"MixedId": {APIName: "MixedId", Type: FieldString, Label: "Existing field"},
	}}
	mergeStandardSObjectStubReferencesFromV2Describe(&definition, nil)
	if _, exists := definition.Fields["UnavailableId"]; exists {
		t.Fatal("unavailable optional reference was projected")
	}
	if field := definition.Fields["MixedId"]; field.Type != FieldString || field.Label != "Existing field" || len(field.ReferenceTo) != 0 {
		t.Fatalf("projection changed an existing field or narrowed a mixed target list: %#v", field)
	}
	if _, exists := definition.Fields["HiddenId"]; exists {
		t.Fatal("hidden field was projected")
	}
	field := definition.Fields["SupportedId"]
	if field.Type != FieldReference || !slices.Equal(field.ReferenceTo, []string{"Account", "Contact"}) || field.RelationshipName != "Supported" || field.Nillable == nil || *field.Nillable || field.Createable == nil || !*field.Createable || field.DefaultedOnCreate == nil || *field.DefaultedOnCreate || field.Required {
		t.Fatalf("supported reference facets changed: %#v", field)
	}
	definition.Fields["SupportedId"] = Field{APIName: "SupportedId", Type: FieldReference, Required: true}
	mergeStandardSObjectStubReferencesFromV2Describe(&definition, nil)
	if !definition.Fields["SupportedId"].Required {
		t.Fatal("explicit DML requirement was removed")
	}
	if field, exists := definition.Fields["UnspecifiedId"]; !exists || field.Type != FieldReference {
		t.Fatal("reference without target constraints was removed")
	}
	plan := buildStandardOverlayCheckPlan(definition.APIName)
	if plan.stubReferenceFieldsNeedRepair(definition, "") || legacyStandardStubReferenceFieldsNeedRepair(definition, nil) {
		t.Fatal("unavailable optional targets requested repeated projection")
	}
	delete(definition.Fields, "SupportedId")
	if !plan.stubReferenceFieldsNeedRepair(definition, "") || !legacyStandardStubReferenceFieldsNeedRepair(definition, nil) {
		t.Fatal("missing supported reference did not request repair")
	}
}

func TestStandardStubReferenceShapesProjectFromDescribe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		objectName        string
		fieldName         string
		referenceTargets  []string
		relationshipName  string
		checkRelationship bool
	}{
		{objectName: "AssociatedLocation", fieldName: "LocationId", referenceTargets: []string{"Location"}, relationshipName: "Location", checkRelationship: true},
		{objectName: "Location", fieldName: "VisitorAddressId", referenceTargets: []string{"Address"}, relationshipName: "VisitorAddress", checkRelationship: true},
		{objectName: "Visit", fieldName: "PlaceId", referenceTargets: []string{"Address", "ContactPointAddress", "Location"}, relationshipName: "Place"},
	}
	for _, tt := range tests {
		t.Run(tt.objectName+"."+tt.fieldName, func(t *testing.T) {
			definition, ok := StandardObjectDefinition(tt.objectName)
			if !ok {
				t.Fatalf("standard definition for %s not found", tt.objectName)
			}
			fieldName, ok := ResolveFieldName(definition, "", tt.fieldName)
			if !ok {
				t.Fatalf("field %s.%s not found", tt.objectName, tt.fieldName)
			}
			field := definition.Fields[fieldName]
			if field.Type != FieldReference || !sameNames(field.ReferenceTo, tt.referenceTargets) {
				t.Fatalf("field = %#v, want reference to %v", field, tt.referenceTargets)
			}
			if field.RelationshipName != tt.relationshipName {
				t.Fatalf("relationship name = %q, want %q", field.RelationshipName, tt.relationshipName)
			}
			if tt.checkRelationship && !containsStandardRelationshipForProjection(definition.Relations, tt.fieldName, tt.relationshipName, tt.referenceTargets[0]) {
				t.Fatalf("relationship %s for %s.%s not found", tt.relationshipName, tt.objectName, tt.fieldName)
			}
		})
	}
	definition, ok := StandardObjectDefinition("Location")
	if !ok {
		t.Fatal("standard definition for Location not found")
	}
	nameField := definition.Fields["Name"]
	if nameField.Type != FieldString {
		t.Fatalf("unrelated Name field type = %q, want STRING", nameField.Type)
	}
	if _, ok := ResolveFieldName(definition, "", "UnmodeledRelationId"); ok {
		t.Fatal("unmodeled reference field was inferred from its name")
	}
}

func TestStandardStubReferenceOverlayRefreshesMissingFields(t *testing.T) {
	t.Parallel()
	definition, ok := StandardObjectDefinition("Location")
	if !ok {
		t.Fatal("standard definition for Location not found")
	}
	delete(definition.Fields, "VisitorAddressId")
	for index := len(definition.Relations) - 1; index >= 0; index-- {
		if strings.EqualFold(definition.Relations[index].Field, "VisitorAddressId") {
			definition.Relations = append(definition.Relations[:index], definition.Relations[index+1:]...)
		}
	}
	if !StandardObjectFieldsNeedWrite(definition) {
		t.Fatal("missing described reference field did not request overlay refresh")
	}
	EnsureStandardObjectFields(&definition)
	name, ok := ResolveFieldName(definition, "", "VisitorAddressId")
	if !ok || definition.Fields[name].Type != FieldReference {
		t.Fatalf("refreshed VisitorAddressId = %#v, want reference field", definition.Fields[name])
	}
}

func sameNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	slices.SortFunc(got, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	slices.SortFunc(want, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	for i := range got {
		if !strings.EqualFold(got[i], want[i]) {
			return false
		}
	}
	return true
}

func containsStandardRelationshipForProjection(relationships []Relationship, fieldName, relationshipName, parent string) bool {
	for _, relationship := range relationships {
		if strings.EqualFold(relationship.Field, fieldName) && strings.EqualFold(relationship.ParentRelationship, relationshipName) && len(relationship.ParentObjects) == 1 && strings.EqualFold(relationship.ParentObjects[0], parent) {
			return true
		}
	}
	return false
}

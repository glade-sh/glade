package storage

import (
	"encoding/json"
	"testing"
)

// P11/org.tsv:2577,2586,2602 and 12066,12075,12091 capture these Case
// fields at API 62/67; P0's named maps omit them. Explicit project metadata
// must preserve their membership without adding them to a bare catalog.
func TestStandardFieldPolicyConfiguredCaseProvenanceRoutes(t *testing.T) {
	for _, field := range []Field{
		{APIName: "HasCommentsUnreadByOwner", Type: FieldBoolean},
		{APIName: "IsVisibleInSelfService", Type: FieldBoolean},
		{APIName: "SourceId", Type: FieldReference, ReferenceTo: []string{"EmailMessage"}, RelationshipName: "Source"},
	} {
		for _, route := range []string{"configured-ensure", "configured-refresh", "configured-json-feature-transition", "bare-catalog", "bare-refresh"} {
			t.Run(field.APIName+"/"+route, func(t *testing.T) {
				configured := route != "bare-catalog" && route != "bare-refresh"
				definition := ObjectDefinition{APIName: "Case"}
				if configured {
					definition.Fields = map[string]Field{
						field.APIName:       field,
						"OwnedUnrelated__c": {APIName: "OwnedUnrelated__c", Type: FieldString, Label: "Owned unrelated field"},
					}
				}
				EnsureStandardObjectFieldsForFeatures(&definition, nil)
				if route == "configured-refresh" || route == "bare-refresh" {
					delete(definition.Fields, "Id")
					if !RefreshStandardObjectFieldOverlay(&definition) {
						t.Fatal("damaged persisted overlay was not refreshed")
					}
				}
				if route == "configured-json-feature-transition" {
					raw, err := json.Marshal(definition)
					if err != nil {
						t.Fatal(err)
					}
					var restored ObjectDefinition
					if err := json.Unmarshal(raw, &restored); err != nil {
						t.Fatal(err)
					}
					definition = restored
					EnsureStandardObjectFieldsForFeatures(&definition, []string{"MultiCurrency"})
					EnsureStandardObjectFieldsForFeatures(&definition, nil)
				}
				if _, exists := ResolveFieldName(definition, "", field.APIName); exists != configured {
					t.Fatalf("configured=%t; %s membership=%t", configured, field.APIName, exists)
				}
				if configured {
					unrelated, exists := definition.Fields["OwnedUnrelated__c"]
					if !exists || unrelated.Type != FieldString || unrelated.Label != "Owned unrelated field" {
						t.Fatalf("unrelated configured field changed: %+v", unrelated)
					}
				}
				if StandardObjectFieldsNeedWrite(definition) {
					t.Fatal("completed overlay repeatedly requests repair")
				}
			})
		}
	}
}

// standard_recordtype_field_policy/org.tsv:2-4,9-11 pins baseline absence.
// Describe conversions can regenerate the reference from the Master record
// type after an overlay; a later ensure must not relabel that field configured.
func TestStandardFieldPolicyGeneratedRecordTypeRepeatedEnsureRoute(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account"}
	EnsureStandardObjectFieldsForFeatures(&definition, nil)
	for pass := 0; pass < 3; pass++ {
		definition.Fields["RecordTypeId"] = Field{
			APIName: "RecordTypeId", Type: FieldReference,
			ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType",
		}
		EnsureStandardObjectFieldsForFeatures(&definition, nil)
		if _, exists := ResolveFieldName(definition, "", "RecordTypeId"); exists {
			t.Fatalf("ensure %d preserves a generated baseline RecordTypeId", pass+1)
		}
		if StandardObjectFieldsNeedWrite(definition) {
			t.Fatalf("ensure %d still requests repair", pass+1)
		}
	}
}

// The same configured lists must remain allocation-free on both direct
// feature checks and the public steady-state overlay predicate.
func TestStandardFieldPolicyConfiguredProvenanceZeroAllocationRoutes(t *testing.T) {
	for _, initial := range []ObjectDefinition{
		{APIName: "Case", Fields: map[string]Field{
			"HasCommentsUnreadByOwner": {APIName: "HasCommentsUnreadByOwner", Type: FieldBoolean},
			"IsVisibleInSelfService":   {APIName: "IsVisibleInSelfService", Type: FieldBoolean},
			"SourceId":                 {APIName: "SourceId", Type: FieldReference, ReferenceTo: []string{"EmailMessage"}, RelationshipName: "Source"},
		}},
		{APIName: "Account", Fields: map[string]Field{
			"RecordTypeId": {APIName: "RecordTypeId", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
		}},
	} {
		definition := initial.Clone()
		EnsureStandardObjectFieldsForFeatures(&definition, nil)
		for _, route := range []string{"feature-check", "overlay-check"} {
			t.Run(initial.APIName+"/"+route, func(t *testing.T) {
				check := func() bool { return standardOptionalFieldsNeedRepair(definition, "") }
				if route == "overlay-check" {
					check = func() bool { return StandardObjectFieldsNeedWrite(definition) }
				}
				if check() {
					t.Fatal("configured definition requests repair")
				}
				if allocations := testing.AllocsPerRun(100, func() { check() }); allocations != 0 {
					t.Fatalf("allocations=%g, want 0", allocations)
				}
			})
		}
	}
}

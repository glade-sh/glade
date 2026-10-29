package storage

import (
	"encoding/json"
	"os"
	"testing"
)

func TestStandardRecordTypeFieldPolicyNativeCatalogAndRefreshRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/standard_recordtype_field_policy/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ API, Profile, Route, Message string }
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 20 {
		t.Fatal("incomplete native record-type matrix")
	}
	for _, row := range rows {
		if row.Route != "anonymous" && row.Route != "named" && row.Route != "test-runner" {
			continue
		}
		for _, route := range []string{"catalog-import", "persisted-refresh"} {
			t.Run(row.API+"/"+row.Profile+"/"+row.Route+"/"+route, func(t *testing.T) {
				definition := ObjectDefinition{APIName: "Account"}
				if route == "persisted-refresh" {
					EnsureStandardObjectFieldsForFeatures(&definition, nil)
				}
				if row.Profile == "recordtype" {
					definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "OwnedPolicy", Name: "Owned policy", Active: true}}
				}
				if route == "persisted-refresh" {
					RefreshStandardObjectFieldOverlay(&definition)
				} else {
					EnsureStandardObjectFieldsForFeatures(&definition, nil)
				}
				field, exists := ResolveFieldName(definition, "", "RecordTypeId")
				want := row.Message != "MISSING"
				if exists != want {
					t.Fatalf("native membership=%t; Glade=%t", want, exists)
				}
				if exists {
					facet := definition.Fields[field]
					if facet.Type != FieldReference || !FieldFlagValue(facet.Createable, true) || !FieldFlagValue(facet.Updateable, true) || len(facet.ReferenceTo) != 1 || facet.ReferenceTo[0] != "RecordType" {
						t.Fatalf("native %s; Glade %+v", row.Message, facet)
					}
				}
			})
		}
	}
}

func TestStandardRecordTypeFieldPolicyFeatureTransitionRoutes(t *testing.T) {
	for _, projectRecordType := range []bool{false, true} {
		name := "feature-record-types-only"
		definition := ObjectDefinition{APIName: "Account"}
		if projectRecordType {
			name = "project-and-feature-record-types"
			definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "OwnedPolicy", Name: "Owned policy", Active: true}}
		}
		t.Run(name, func(t *testing.T) {
			EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
			EnsureStandardObjectFieldsForFeatures(&definition, nil)
			if _, exists := ResolveFieldName(definition, "", "RecordTypeId"); exists != projectRecordType {
				t.Fatalf("project record type=%t; field membership=%t", projectRecordType, exists)
			}
			owned := false
			for _, recordType := range definition.RecordTypes {
				if recordType.DeveloperName == "PersonAccount" {
					t.Fatal("disabled feature retains its generated record type")
				}
				owned = owned || recordType.DeveloperName == "OwnedPolicy"
			}
			if owned != projectRecordType {
				t.Fatal("project record type changed with feature profile")
			}
			if StandardObjectFieldsNeedWrite(definition) {
				t.Fatal("completed profile transition still requests repair")
			}
		})
	}
}

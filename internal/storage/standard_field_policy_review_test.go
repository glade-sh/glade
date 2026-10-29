package storage

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandardFieldPolicyConfiguredImportAndPersistedRefreshRoutes(t *testing.T) {
	for _, tc := range []struct {
		object, field string
		kind          FieldType
	}{
		{"Account", "RecordTypeId", FieldReference}, {"Opportunity", "RecordTypeId", FieldReference}, {"EmailMessage", "ToIds", FieldAny},
	} {
		for _, route := range []string{"configured-import", "persisted-refresh", "persisted-current-marker", "persisted-unrelated-feature-marker"} {
			t.Run(tc.object+"."+tc.field+"/"+route, func(t *testing.T) {
				field := Field{APIName: tc.field, Type: tc.kind}
				if tc.field == "RecordTypeId" {
					field.ReferenceTo = []string{"RecordType"}
					field.RelationshipName = "RecordType"
				}
				def := ObjectDefinition{APIName: tc.object, Fields: map[string]Field{tc.field: field}}
				if route == "persisted-current-marker" {
					snapshotConfiguredStandardPolicyFields(&def)
					def.Metadata[standardFieldsOverlayMarker] = ""
				}
				if route == "persisted-unrelated-feature-marker" {
					snapshotConfiguredStandardPolicyFields(&def)
					def.Metadata[standardFieldsOverlayMarker] = "multicurrency"
				}
				EnsureStandardObjectFieldsForFeatures(&def, nil)
				if route == "persisted-refresh" {
					def.Fields["Name"] = Field{APIName: "Name"}
					RefreshStandardObjectFieldOverlay(&def)
				}
				if _, ok := ResolveFieldName(def, "", tc.field); !ok {
					t.Fatalf("configured %s was deleted", tc.field)
				}
				if StandardObjectFieldsNeedWrite(def) {
					t.Fatal("configured definition repeatedly requests repair")
				}
			})
		}
	}
}

func TestStandardFieldPolicyFeatureCheckZeroAllocationRoutes(t *testing.T) {
	for _, object := range []string{"Account", "Contact", "Case", "Opportunity", "EmailMessage"} {
		def := ObjectDefinition{APIName: object}
		EnsureStandardObjectFieldsForFeatures(&def, nil)
		for _, route := range []string{"feature-check", "overlay-check"} {
			t.Run(object+"/"+route, func(t *testing.T) {
				check := func() { standardOptionalFieldsNeedRepair(def, "") }
				if route == "overlay-check" {
					check = func() { StandardObjectFieldsNeedWrite(def) }
				}
				check()
				if n := testing.AllocsPerRun(100, check); n != 0 {
					t.Fatalf("allocations %.0f, want 0", n)
				}
			})
		}
	}
}

func TestStandardFieldPolicyNativeToIdsConstructorRoute(t *testing.T) {
	if !StandardObjectConstructorFieldAllowedForFeatures("EmailMessage", "ToIds", nil, ObjectDefinition{}) {
		t.Fatal("native accepts ToIds constructor outside describe map")
	}
	if StandardObjectConstructorFieldAllowedForFeatures("Account", "UnknownOwnedField", nil, ObjectDefinition{}) {
		t.Fatal("unknown Account constructor field accepted")
	}
}

// A constructor-only field must never turn into a describe field on a bare
// catalog object. Configured schema and the native constructor are separate routes.
func TestStandardFieldPolicyNativeToIdsConstructorAndDescribeRouteTwins(t *testing.T) {
	for _, field := range []string{"ToIds", "toids", "TOIDS"} {
		if !StandardObjectConstructorFieldAllowedForFeatures("EmailMessage", field, nil, ObjectDefinition{}) {
			t.Fatalf("native constructor rejects %s", field)
		}
	}
	definition := ObjectDefinition{APIName: "EmailMessage"}
	EnsureStandardObjectFieldsForFeatures(&definition, nil)
	if _, ok := ResolveFieldName(definition, "", "ToIds"); ok {
		t.Fatal("constructor-only ToIds leaked into default describe")
	}
	if StandardObjectConstructorFieldAllowedForFeatures("EmailMessage", "UnknownOwnedField", nil, ObjectDefinition{}) {
		t.Fatal("native unknown-field rejection bypassed")
	}
}

func TestStandardFieldPolicyRetainedNativeAbsentCatalogAndRefreshRoutes(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/standard_field_policy_review/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 11 {
		t.Fatal("incomplete retained native membership matrix")
	}
	for _, row := range rows[1:] {
		raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance", row[3]))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		if hex.EncodeToString(digest[:]) != row[4] || row[2] != "false" {
			t.Fatal("native membership input changed")
		}
		object, field, _ := strings.Cut(row[1], ".")
		for _, route := range []string{"catalog-import", "persisted-refresh"} {
			t.Run(row[0]+"/"+row[1]+"/"+route, func(t *testing.T) {
				definition := ObjectDefinition{APIName: object}
				EnsureStandardObjectFieldsForFeatures(&definition, nil)
				if route == "persisted-refresh" {
					definition.Fields[field] = Field{APIName: field, Type: FieldString}
					RefreshStandardObjectFieldOverlay(&definition)
				}
				if _, exists := ResolveFieldName(definition, "", field); exists {
					t.Fatalf("native absent %s remains", row[1])
				}
			})
		}
	}
}

func TestStandardFieldPolicyConfiguredJSONRoundTripRefreshRoute(t *testing.T) {
	for _, tc := range []struct {
		object, field string
		value         Field
	}{
		{"Account", "RecordTypeId", Field{APIName: "RecordTypeId", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}},
		{"EmailMessage", "ToIds", Field{APIName: "ToIds", Type: FieldAny}},
	} {
		t.Run(tc.object+"."+tc.field, func(t *testing.T) {
			definition := ObjectDefinition{APIName: tc.object, Fields: map[string]Field{tc.field: tc.value}}
			EnsureStandardObjectFieldsForFeatures(&definition, nil)
			raw, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			var restored ObjectDefinition
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			restored.Fields["Id"] = Field{APIName: "Id", Type: FieldID}
			EnsureStandardObjectFieldsForFeatures(&restored, []string{"MultiCurrency"})
			EnsureStandardObjectFieldsForFeatures(&restored, nil)
			if _, ok := ResolveFieldName(restored, "", tc.field); !ok {
				t.Fatal("configured field lost across persisted profile transitions")
			}
		})
	}
}

func TestStandardFieldPolicyLegacyConfiguredAndGeneratedProfileRouteTwins(t *testing.T) {
	for _, configured := range []bool{false, true} {
		route := "generated-recordtype-with-snapshot"
		if configured {
			route = "configured-reference-with-snapshot"
		}
		t.Run(route, func(t *testing.T) {
			def := ObjectDefinition{APIName: "Account"}
			if configured {
				def.Fields = map[string]Field{"RecordTypeId": {APIName: "RecordTypeId", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}}
			}
			EnsureStandardObjectFieldsForFeatures(&def, []string{"PersonAccounts"})
			EnsureStandardObjectFieldsForFeatures(&def, nil)
			if _, ok := ResolveFieldName(def, "", "RecordTypeId"); ok != configured {
				t.Fatalf("configured=%t; RecordTypeId exists=%t", configured, ok)
			}
		})
	}
}

package storage

import (
	"encoding/json"
	"sort"
	"testing"
)

// Configuration is independent of catalog availability. Exercise every gate
// (including legacy and compound gates), plus the six reported migration losses.
// Native membership: standard_describe_profiles/{P1,P2,P3,P11}/org.tsv;
// RecordTypeId: standard_recordtype_field_policy/org.tsv:16-25;
// ToIds: standard_emailmessage_input_fields/org.tsv (input-only, not describe).
func TestStandardFieldPolicyConfiguredEveryGateMigration(t *testing.T) {
	objects := map[string]map[string]Field{}
	add := func(object string, field Field) {
		if objects[object] == nil {
			objects[object] = map[string]Field{}
		}
		objects[object][field.APIName] = field
	}
	for _, policy := range standardFieldPolicies {
		for _, gate := range policy.gates {
			add(gate.Object, describeField(gate.Field))
		}
	}
	for _, field := range []string{"HasCommentsUnreadByOwner", "IsVisibleInSelfService", "SourceId"} {
		add("Case", Field{APIName: field, Type: FieldString})
	}
	add("Account", Field{APIName: "RecordTypeId", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"})
	add("EmailMessage", Field{APIName: "ToIds", Type: FieldAny})
	add("Contact", Field{APIName: "CurrencyIsoCode", Type: FieldString})
	names := make([]string, 0, len(objects))
	for name := range objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, object := range names {
		for _, route := range []string{"new", "legacy-ensure", "legacy-refresh", "legacy-json"} {
			t.Run(object+"/"+route, func(t *testing.T) {
				def := ObjectDefinition{APIName: object, Fields: objects[object]}.Clone()
				if route != "new" {
					// Schemas written before provenance snapshots have only this marker.
					def.Metadata = map[string]string{standardFieldsOverlayMarker: ""}
				}
				if route == "legacy-json" {
					raw, err := json.Marshal(def)
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(raw, &def); err != nil {
						t.Fatal(err)
					}
				}
				if route == "legacy-refresh" {
					if !RefreshStandardObjectFieldOverlay(&def) {
						t.Fatal("legacy overlay was not refreshed")
					}
				} else {
					EnsureStandardObjectFieldsForFeatures(&def, nil)
				}
				for _, features := range [][]string{{"MultiCurrency", "PersonAccounts", "StateAndCountryPicklist", "Quotes", "HealthCloud"}, nil} {
					EnsureStandardObjectFieldsForFeatures(&def, features)
					for field := range objects[object] {
						if _, exists := ResolveFieldName(def, "", field); !exists {
							t.Errorf("configured %s.%s lost under %v", object, field, features)
						}
					}
				}
				if StandardObjectFieldsNeedWrite(def) {
					t.Fatal("completed migration requests another repair")
				}
			})
		}
	}
}

// Non-Master metadata activates the reference independently of baseline absence.
// Account native positive and Master-only control: standard_recordtype_field_policy/
// org.tsv:2-25. The same metadata rule covers every absence-table object.
func TestStandardFieldPolicyRecordTypeMetadataEveryAbsentObject(t *testing.T) {
	names := []string{"Account"}
	for name, policy := range standardFieldPolicies {
		if policy.absent["recordtypeid"] {
			canonical, ok := ResolveKnownStandardObjectName(name)
			if !ok {
				t.Fatalf("unknown object %s", name)
			}
			names = append(names, canonical)
		}
	}
	if len(names) != 15 {
		t.Fatalf("record-type matrix has %d objects, want Account plus 14", len(names))
	}
	sort.Strings(names)
	for _, object := range names {
		for _, route := range []string{"direct", "overlay", "warm-overlay", "refresh"} {
			for _, configured := range []bool{false, true} {
				shape := "master-only"
				if configured {
					shape = "non-master"
				}
				t.Run(object+"/"+route+"/"+shape, func(t *testing.T) {
					def := ObjectDefinition{APIName: object, RecordTypes: []RecordTypeInfo{{DeveloperName: "Master"}}}
					if route == "warm-overlay" || route == "refresh" {
						EnsureStandardObjectFieldsForFeatures(&def, nil)
					}
					if configured {
						def.RecordTypes = append(def.RecordTypes, RecordTypeInfo{DeveloperName: "Owned", Name: "Owned", Active: true})
					}
					switch route {
					case "direct":
						EnsureRecordTypeIDField(&def)
					case "refresh":
						RefreshStandardObjectFieldOverlay(&def)
					default:
						EnsureStandardObjectFieldsForFeatures(&def, nil)
					}
					if _, exists := ResolveFieldName(def, "", "RecordTypeId"); exists != configured {
						t.Fatalf("configured=%t; RecordTypeId=%t", configured, exists)
					}
					if configured && !StandardObjectConstructorFieldAllowedForFeatures(object, "RecordTypeId", nil, def) {
						t.Fatal("effective field rejected in constructor")
					}
					if route != "direct" && StandardObjectFieldsNeedWrite(def) {
						t.Fatal("record-type overlay requests another repair")
					}
					if configured {
						def.RecordTypes = []RecordTypeInfo{{DeveloperName: "Master"}}
						EnsureStandardObjectFieldsForFeatures(&def, nil)
						if _, exists := ResolveFieldName(def, "", "RecordTypeId"); exists {
							t.Fatal("generated reference survived removal of non-Master metadata")
						}
					}
				})
			}
		}
	}
}

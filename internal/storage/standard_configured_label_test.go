package storage

import (
	"encoding/json"
	"testing"
)

// These are local configured-schema controls, not native label claims.
// Clean-org absence is captured in standard_describe_profiles/P0/
// historical-extra-fields.tsv:2,20,404,418,436,820 (APIs 62 and 67).
func TestConfiguredOptionalStandardFieldLabelsSurviveOverlay(t *testing.T) {
	for _, row := range []struct{ object, field string }{
		{"Account", "Division"}, {"Case", "Division"}, {"User", "DefaultDivision"},
	} {
		t.Run(row.object+"."+row.field, func(t *testing.T) {
			def := ObjectDefinition{APIName: row.object, Fields: map[string]Field{
				row.field: {APIName: row.field, Label: row.field, Type: FieldPicklist},
			}}
			for _, features := range [][]string{nil, {"__glade_unspecified_standard_field_shape"}, nil} {
				EnsureStandardObjectFieldsForFeatures(&def, features)
				if got := def.Fields[row.field].Label; got != row.field {
					t.Fatalf("configured label = %q, want %q", got, row.field)
				}
				if StandardObjectFieldsNeedWrite(def) {
					t.Fatal("completed overlay still needs repair")
				}
				if RefreshStandardObjectFieldOverlay(&def) {
					t.Fatal("warm refresh changed completed overlay")
				}

			}
		})
	}
}

// Legacy overlays retain their fields for membership, but their label origins
// are unknown. An API-name placeholder must still be repaired on every route.
func TestLegacyOptionalFieldPlaceholderLabelsAreRepaired(t *testing.T) {
	for _, row := range []struct{ object, field, label string }{
		{"Account", "Division", "Division ID"},
		{"Case", "Division", "Division ID"},
		{"User", "DefaultDivision", "Division ID"},
	} {
		for _, route := range []string{"ensure", "refresh", "old-snapshot"} {
			t.Run(row.object+"."+row.field+"/"+route, func(t *testing.T) {
				def := ObjectDefinition{APIName: row.object}
				EnsureStandardObjectFieldsForFeatures(&def, []string{"__glade_unspecified_standard_field_shape"})
				field := def.Fields[row.field]
				if field.APIName == "" {
					t.Fatal("missing generated field")
				}
				field.Label = row.field
				def.Fields[row.field] = field
				delete(def.Metadata, standardConfiguredPolicyFields)
				delete(def.Metadata, "__glade_standard_configured_policy_fields_label_provenance")
				if route == "old-snapshot" {
					def.Metadata[standardConfiguredPolicyFields] = row.field
				}
				if route == "refresh" || route == "old-snapshot" {
					if !RefreshStandardObjectFieldOverlay(&def) {
						t.Fatal("legacy label did not request repair")
					}
				} else {
					EnsureStandardObjectFieldsForFeatures(&def, []string{"__glade_unspecified_standard_field_shape"})
				}
				if got := def.Fields[row.field].Label; got != row.label {
					t.Fatalf("label = %q, want %q", got, row.label)
				}
				if StandardObjectFieldsNeedWrite(def) {
					t.Fatal("completed overlay still needs repair")
				}
				if RefreshStandardObjectFieldOverlay(&def) {
					t.Fatal("warm refresh changed completed overlay")
				}
				data, err := json.Marshal(def)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &def); err != nil {
					t.Fatal(err)
				}
				field = def.Fields[row.field]
				field.Label = row.field
				def.Fields[row.field] = field
				if !RefreshStandardObjectFieldOverlay(&def) || def.Fields[row.field].Label != row.label {
					t.Fatal("persisted legacy overlay did not repair a recurring placeholder")
				}
				field = def.Fields[row.field]
				field.Label = "Configured label"
				def.Fields[row.field] = field
				EnsureStandardObjectFieldsForFeatures(&def, nil)
				if def.Fields[row.field].Label != "Configured label" {
					t.Fatal("explicit distinct label was overwritten")
				}
			})
		}
	}
}

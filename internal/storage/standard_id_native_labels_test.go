package storage

import (
	"encoding/json"
	"fmt"
	"testing"
)

// P0 API 62 granted describes: Lead, Account, and Contact Id labels.
func TestStandardIDLabelsUseNativeCatalogAndPreserveConfigured(t *testing.T) {
	for _, tc := range []struct{ object, label string }{{"Lead", "Lead ID"}, {"Account", "Account ID"}, {"Contact", "Contact ID"}} {
		t.Run(tc.object, func(t *testing.T) {
			def, ok := StandardObjectDefinition(tc.object)
			if !ok {
				t.Fatal("missing object")
			}
			if got := def.Fields["Id"].Label; got != tc.label {
				t.Fatalf("native Id label=%q want %q", got, tc.label)
			}
			configured := ObjectDefinition{APIName: tc.object, Fields: map[string]Field{"Id": {APIName: "Id", Type: FieldID, Label: "Owned ID"}}}
			EnsureStandardObjectFields(&configured)
			if got := configured.Fields["Id"].Label; got != "Owned ID" {
				t.Fatalf("configured label overwritten: %q", got)
			}
		})
	}
}

func TestStandardIDLabelRefreshPreservesProjectProvenance(t *testing.T) {
	for _, tc := range []struct{ object, label string }{{"Account", "Account ID"}, {"Lead", "Lead ID"}, {"Order", "Order ID"}} {
		for _, configured := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/configured=%v", tc.object, configured), func(t *testing.T) {
				def, ok := StandardObjectDefinition(tc.object)
				if !ok {
					t.Fatal("missing object")
				}
				field := def.Fields["Id"]
				field.Label = "Record ID"
				def.Fields["Id"] = field
				def.Metadata[standardConfiguredPolicyFieldsLabelProvenance] = "true"
				def.Metadata[standardConfiguredPolicyFields] = ""
				if configured {
					def.Metadata[standardConfiguredPolicyFields] = "Id"
				}
				data, err := json.Marshal(def)
				if err != nil {
					t.Fatal(err)
				}
				var restored ObjectDefinition
				if err = json.Unmarshal(data, &restored); err != nil {
					t.Fatal(err)
				}
				RefreshStandardObjectFieldOverlay(&restored)
				want := tc.label
				if configured {
					want = "Record ID"
				}
				if got := restored.Fields["Id"].Label; got != want {
					t.Fatalf("Id label=%q want=%q", got, want)
				}
			})
		}
	}
}

func TestStandardIDLabelRefreshPreservesUnknownLegacyProvenance(t *testing.T) {
	def, ok := StandardObjectDefinition("Lead")
	if !ok {
		t.Fatal("missing Lead")
	}
	field := def.Fields["Id"]
	field.Label = "Record ID"
	def.Fields["Id"] = field
	delete(def.Metadata, standardConfiguredPolicyFieldsLabelProvenance)
	RefreshStandardObjectFieldOverlay(&def)
	if got := def.Fields["Id"].Label; got != "Record ID" {
		t.Fatalf("unknown legacy label overwritten: %q", got)
	}
}

func TestStandardIDLabelLeavesUnprovenObjectsOnLegacyBaseline(t *testing.T) {
	for _, object := range []string{"AuthProvider", "WebLink", "ApexTestResult"} {
		t.Run(object, func(t *testing.T) {
			def, ok := StandardObjectDefinition(object)
			if !ok {
				t.Fatal("missing standard object")
			}
			if got := def.Fields["Id"].Label; got != "Record ID" {
				t.Fatalf("uncaptured Id label=%q want legacy Record ID", got)
			}
		})
	}
}

package dml

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHierarchySettingOrganizationNameDefault(t *testing.T) {
	org := storage.NewOrgState()
	org.OrgID = "00D000000000mH2"
	definition := storage.ObjectDefinition{
		APIName: "TriadPreference__c", Label: "Triad Preference",
		Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
		Fields: map[string]storage.Field{
			"Name":         {APIName: "Name", Type: storage.FieldString},
			"SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldReference},
		},
	}
	for _, tc := range []struct {
		name   string
		fields map[string]storage.Value
		want   string
	}{
		{"omitted owner", nil, "Triad Preference (Organization)"},
		{"organization string", map[string]storage.Value{"SetupOwnerId": storage.StringValue(org.OrgID)}, "Triad Preference (Organization)"},
		{"organization Id", map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(org.OrgID))}, "Triad Preference (Organization)"},
		{"explicit name", map[string]storage.Value{"Name": storage.StringValue("Chosen"), "SetupOwnerId": storage.StringValue(org.OrgID)}, "Chosen"},
		{"user unchanged", map[string]storage.Value{"SetupOwnerId": storage.StringValue("005000000000001")}, org.OrgID},
		{"profile unchanged", map[string]storage.Value{"SetupOwnerId": storage.StringValue("00e000000000001")}, org.OrgID},
		{"null name unchanged", map[string]storage.Value{"Name": storage.NullValue()}, org.OrgID},
		{"empty name unchanged", map[string]storage.Value{"Name": storage.StringValue("")}, org.OrgID},
		// Scope controls retain existing behavior without a native parity claim.
		{"null owner unchanged", map[string]storage.Value{"SetupOwnerId": storage.NullValue()}, "Triad Preference (Organization)"},
		{"empty owner unchanged", map[string]storage.Value{"SetupOwnerId": storage.StringValue("")}, "Triad Preference (Organization)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := storage.Record{Object: definition.APIName, Fields: tc.fields}
			applyCustomSettingInsertDefaults(&org, definition, &record)
			if record.HierarchyNameUsesRecordID != (tc.name == "omitted owner") {
				t.Fatalf("deferred record-ID name = %v", record.HierarchyNameUsesRecordID)
			}
			if record.HierarchyNameUsesRecordID && record.Fields["SetupOwnerId"].Kind != storage.ValueID {
				t.Fatal("declared reference owner lost its Id type")
			}
			// Name is a validation placeholder until insert allocates the ID.
			if got := record.Fields["Name"].String; got != tc.want {
				t.Fatalf("Name = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHierarchyNameProvenanceSurvivesStorageAndClearsOnExplicitUpdate(t *testing.T) {
	for _, field := range []string{"Tone__c", "Name", "SetupOwnerId"} {
		t.Run(field, func(t *testing.T) {
			org := storage.NewOrgState()
			definition := storage.ObjectDefinition{
				APIName: "TriadPreference__c", Label: "Triad Preference", KeyPrefix: "a02",
				Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
				Fields: map[string]storage.Field{
					"Name":         {APIName: "Name", Type: storage.FieldString},
					"SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldString},
					"Tone__c":      {APIName: "Tone__c", Type: storage.FieldString},
				},
			}
			org.Objects[definition.APIName] = storage.ObjectState{Definition: definition}
			engine := NewEngine(&org)
			insert := engine.Insert([]storage.Record{{Object: definition.APIName}})
			if len(insert) != 1 || !insert[0].Success {
				t.Fatalf("insert: %+v", insert)
			}
			stored := org.Objects[definition.APIName].Records[insert[0].ID].Clone()
			raw, err := json.Marshal(stored)
			if err != nil {
				t.Fatal(err)
			}
			var restored storage.Record
			if err := json.Unmarshal(raw, &restored); err != nil || !restored.HierarchyNameUsesRecordID {
				t.Fatalf("stored name provenance lost: %v", err)
			}
			object := org.Objects[definition.APIName]
			object.Records[insert[0].ID] = restored
			org.Objects[definition.APIName] = object
			value := storage.StringValue("changed")
			if field == "SetupOwnerId" {
				value = storage.StringValue("00D000000000mH2")
			}
			updated := engine.Update([]storage.Record{{Object: definition.APIName, ID: insert[0].ID, Fields: map[string]storage.Value{field: value}}})
			if len(updated) != 1 || !updated[0].Success {
				t.Fatalf("update: %+v", updated)
			}
			if got := org.Objects[definition.APIName].Records[insert[0].ID].HierarchyNameUsesRecordID; got != (field == "Tone__c") {
				t.Fatalf("name provenance after %s update = %v", field, got)
			}
		})
	}
}

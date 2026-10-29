package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// commerce_extension_shapes/org.tsv:48,50,60-62,109,111,121-123
// captures the WebStore checkbox shape and false/null read at APIs 62 and 67.
func TestInsertOmittedNonNillableCheckboxes(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "WebStore")
	org.Objects["OwnedCheckbox__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "OwnedCheckbox__c", KeyPrefix: "a90", Fields: map[string]storage.Field{
			"Name":           {APIName: "Name", Type: storage.FieldString, Required: true},
			"First__c":       {APIName: "First__c", Type: storage.FieldBoolean, Required: true, Createable: storage.BoolFlag(true), Nillable: storage.BoolFlag(false)},
			"Second__c":      {APIName: "Second__c", Type: storage.FieldBoolean, Required: true, Createable: storage.BoolFlag(true), Nillable: storage.BoolFlag(false)},
			"Enabled__c":     {APIName: "Enabled__c", Type: storage.FieldBoolean, Required: true, Createable: storage.BoolFlag(true), Nillable: storage.BoolFlag(false)},
			"DefaultTrue__c": {APIName: "DefaultTrue__c", Type: storage.FieldBoolean, Required: true, Createable: storage.BoolFlag(true), Nillable: storage.BoolFlag(false), DefaultValue: "true"},
		}}, Records: map[storage.ID]storage.Record{},
	}
	engine := NewEngine(&org)
	for _, tc := range []struct {
		object string
		fields map[string]storage.Value
		want   map[string]bool
	}{
		{"WebStore", map[string]storage.Value{"Name": storage.StringValue("Owned store")}, map[string]bool{"OptionsAutoFacetingEnabled": false}},
		{"OwnedCheckbox__c", map[string]storage.Value{"Name": storage.StringValue("Owned record"), "Enabled__c": storage.BooleanValue(true)}, map[string]bool{"First__c": false, "Second__c": false, "Enabled__c": true, "DefaultTrue__c": true}},
	} {
		t.Run(tc.object, func(t *testing.T) {
			result := engine.Insert([]storage.Record{{Object: tc.object, Fields: tc.fields}})
			if len(result) != 1 || !result[0].Success {
				t.Fatalf("insert = %#v", result)
			}
			stored := org.Objects[tc.object].Records[result[0].ID]
			for name, want := range tc.want {
				value, exists := stored.GetField(name)
				if !exists || value.Kind != storage.ValueBoolean || value.Boolean != want {
					t.Errorf("%s = %#v, exists=%t; want %t", name, value, exists, want)
				}
			}
		})
	}
	result := engine.Insert([]storage.Record{{Object: "OwnedCheckbox__c"}})
	if len(result) != 1 || result[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("missing required text = %#v", result)
	}
}

package compile_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native org has configured standard value sets and layouts. Export those
// inputs separately from expected wire outputs so CI renders a comparable org,
// rather than the legacy Enterprise catalog's different configuration.
func l10SetupOrgMetadata(t *testing.T, org *storage.OrgState) {
	t.Helper()
	var fixture struct {
		Source  string `json:"source"`
		Objects map[string]struct {
			ComponentLabels map[string]string `json:"componentLabels"`
			Picklists       map[string]struct {
				Values            []storage.PicklistValue `json:"values"`
				DefaultAttributes map[string]any          `json:"defaultAttributes"`
			} `json:"picklists"`
		} `json:"objects"`
	}
	data, err := os.ReadFile("testdata/l10_org_metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Source == "" || len(fixture.Objects) != 5 {
		t.Fatal("missing captured L10 org configuration")
	}
	for objectName, metadata := range fixture.Objects {
		storage.EnsureStandardObject(org, objectName)
		object := org.Objects[objectName]
		object.Definition = object.Definition.Clone()
		for name, field := range object.Definition.Fields {
			if field.Type != storage.FieldPicklist && field.Type != storage.FieldMultiPicklist {
				continue
			}
			if _, exists := metadata.Picklists[name]; !exists {
				// The captured UI catalog has no available values for these fields.
				field.PicklistValues = nil
				field.PicklistValuesConfigured = true
				field.DefaultValue = ""
				object.Definition.Fields[name] = field
			}
		}
		for name, picklist := range metadata.Picklists {
			field, exists := object.Definition.Fields[name]
			if !exists {
				// Native getPicklistValuesByRecordType establishes this field and
				// its value set even when the local catalog does not include it.
				// Leave uncaptured labels and field facets unset.
				field = storage.Field{APIName: name}
			}
			field.Type = storage.FieldPicklist
			field.DisplayType = string(storage.FieldPicklist)
			field.DefaultValue = ""
			field.PicklistValuesConfigured = true
			field.PicklistValues = append([]storage.PicklistValue(nil), picklist.Values...)
			object.Definition.Fields[name] = field
			if len(picklist.DefaultAttributes) == 0 {
				continue
			}
			setupName, ok := picklist.DefaultAttributes["picklistAtrributesValueType"].(string)
			if !ok || (setupName != "CaseStatus" && setupName != "LeadStatus") {
				t.Fatalf("invalid captured status metadata: %s.%s", objectName, name)
			}
			storage.EnsureStandardObject(org, setupName)
			setup := org.Objects[setupName].Clone()
			flag, attr := "IsClosed", "closed"
			if setupName == "LeadStatus" {
				flag, attr = "IsConverted", "converted"
			}
			state, ok := picklist.DefaultAttributes[attr].(bool)
			if !ok {
				t.Fatal("missing captured status flag")
			}
			for _, value := range picklist.Values {
				if !value.Default {
					continue
				}
				for id, record := range setup.Records {
					if record.Fields["MasterLabel"].String == value.Value {
						delete(setup.Records, id)
					}
				}
				if setup.Records == nil {
					setup.Records = map[storage.ID]storage.Record{}
				}
				id := storage.ID(fmt.Sprintf("%s000000000001AAA", storage.StandardKeyPrefix(setupName)))
				setup.Records[id] = storage.Record{ID: id, Object: setupName, Fields: map[string]storage.Value{"MasterLabel": storage.StringValue(value.Value), flag: storage.BooleanValue(state)}}
			}
			org.Objects[setupName] = setup
		}
		for name, label := range metadata.ComponentLabels {
			field, exists := object.Definition.Fields[name]
			if !exists {
				t.Fatalf("captured layout component missing: %s.%s", objectName, name)
			}
			field.Label = label
			object.Definition.Fields[name] = field
		}
		org.Objects[objectName] = object
	}
}

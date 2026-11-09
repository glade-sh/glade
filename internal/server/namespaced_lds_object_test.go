package server

import (
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

// Components in a namespaced package import their own objects unprefixed
// (Widget__c.Size__c inside acme). LDS lookups resolve them like Apex does.
func TestPicklistWireResolvesUnprefixedObjectInOrgNamespace(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "acme"
	org.Objects["acme__Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "acme__Widget__c",
			KeyPrefix: "a0W",
			Fields: map[string]storage.Field{
				"acme__Size__c": {APIName: "acme__Size__c", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Small", Label: "Small"}}},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	data, wireErr := getPicklistValuesWireData(&org, lwcbrowser.WireGetPicklistValuesRequest{FieldAPIName: "Widget__c.Size__c"})
	if wireErr != nil {
		t.Fatalf("picklist error = %#v", wireErr)
	}
	values, _ := data["values"].([]map[string]any)
	if len(values) != 1 || values[0]["value"] != "Small" {
		t.Fatalf("picklist data = %#v", data)
	}
}

func TestGetRecordAcceptsUnprefixedObjectInOrgNamespace(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "acme"
	org.Objects["acme__Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "acme__Widget__c",
			KeyPrefix: "a0W",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0W00000001AAAAAAA": {ID: "a0W00000001AAAAAAA", Object: "acme__Widget__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Blue")}},
		},
	}
	data, wireErr := getRecordWireData(&org, "a0W00000001AAAAAAA", []string{"Widget__c.Name"}, nil)
	if wireErr != nil {
		t.Fatalf("getRecord error = %#v", wireErr)
	}
	fields, _ := data["fields"].(map[string]any)
	name, _ := fields["Name"].(map[string]any)
	if name["value"] != "Blue" {
		t.Fatalf("getRecord fields = %#v", fields)
	}
}

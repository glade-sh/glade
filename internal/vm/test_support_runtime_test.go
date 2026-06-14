package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecTestLoadDataFullLocalNamespaceAndRelationshipExternalID(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{
		Name:            "Children",
		NamespacePrefix: "pkg",
		Content:         "Name,Parent__r.External_Id__c\nChild One,PARENT-1\n",
	}}
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Id":             {APIName: "Id", Type: storage.FieldID},
				"Name":           {APIName: "Name", Type: storage.FieldString},
				"External_Id__c": {APIName: "External_Id__c", Type: storage.FieldString, ExternalID: true, Unique: true},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a10000000000001AAA": {
				ID:     "a10000000000001AAA",
				Object: "Parent__c",
				Fields: map[string]storage.Value{
					"Id":             storage.IDValue("a10000000000001AAA"),
					"Name":           storage.StringValue("Parent"),
					"External_Id__c": storage.StringValue("PARENT-1"),
				},
			},
		},
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	rows, err := machine.testLoadData([]Value{sObjectTypeToken("Child__c"), String("pkg__Children")}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if rows.Kind != ValueList || len(rows.List) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	childObject := machine.Org.Objects["Child__c"]
	if len(childObject.Records) != 1 {
		t.Fatalf("child records = %#v", childObject.Records)
	}
	for _, record := range childObject.Records {
		if got := storageIDFromValue(record.Fields["Parent__c"]); got != "a10000000000001AAA" {
			t.Fatalf("Parent__c = %q", got)
		}
	}
}

func TestExecTestLoadDataFullLocalNamespaceAndRelationshipDiagnostics(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{
		Name:            "Children",
		NamespacePrefix: "pkg",
		Content:         "Name,Parent__r.External_Id__c\nChild One,MISSING\n",
	}}
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Id":             {APIName: "Id", Type: storage.FieldID},
				"External_Id__c": {APIName: "External_Id__c", Type: storage.FieldString, ExternalID: true, Unique: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.testLoadData([]Value{sObjectTypeToken("Child__c"), String("other__Children")}, &Result{}); err == nil || err.Error() != "Test.loadData static resource other__Children namespace other does not match local package namespace pkg" {
		t.Fatalf("bad namespace err = %v", err)
	}
	_, err := machine.testLoadData([]Value{sObjectTypeToken("Child__c"), String("pkg__Children")}, &Result{})
	if err == nil || !strings.Contains(err.Error(), "FIELD_INTEGRITY_EXCEPTION") || !strings.Contains(err.Error(), "Parent__r.External_Id__c") || !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("bad relationship err = %v", err)
	}
}

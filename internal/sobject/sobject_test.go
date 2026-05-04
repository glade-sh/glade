package sobject

import (
	"testing"

	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/storage"
)

func TestValueTracksFieldsAndExplicitNulls(t *testing.T) {
	account := New("Account")
	account.ID = "001000000000001"
	account.Put("Name", storage.StringValue("Acme"))
	account.Put("Description", storage.NullValue())

	if value, ok := account.Get("Name"); !ok || value.String != "Acme" {
		t.Fatalf("Name = %#v ok=%t", value, ok)
	}
	if value, ok := account.Get("Description"); !ok || value.Kind != storage.ValueNull {
		t.Fatalf("Description = %#v ok=%t", value, ok)
	}
	record := account.ToRecord()
	roundTrip := FromRecord(record)
	roundTrip.Put("Name", storage.StringValue("Changed"))
	if original, _ := account.Get("Name"); original.String != "Acme" {
		t.Fatalf("original changed: %#v", original)
	}
}

func TestBuildDescribeRegistry(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
		Name:  "Widget__c",
		Label: "Widget",
		Fields: []schema.Field{
			{Name: "Name", Type: "Text", Required: true},
			{Name: "Account__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Account__r", ChildRelationshipName: "Widgets__r", DeleteConstraint: "Cascade"},
			{Name: "Who__c", Type: "Lookup", ReferenceTo: []string{"Account", "Contact"}, RelationshipName: "Who__r", ChildRelationshipName: "WhoWidgets__r"},
			{Name: "Rating__c", Type: "Picklist", PicklistValues: []schema.PicklistValue{{FullName: "Hot", Label: "Hot", Default: true, Active: true}}},
		},
		RecordTypes: []schema.RecordType{{DeveloperName: "Business", Label: "Business Widget", Active: true, Default: true}},
	}}})

	describe, err := registry.Describe("Widget__c")
	if err != nil {
		t.Fatal(err)
	}
	if describe.KeyPrefix != "a00" {
		t.Fatalf("prefix = %q", describe.KeyPrefix)
	}
	if describe.Fields["Name"].Type != storage.FieldString || !describe.Fields["Name"].Required {
		t.Fatalf("Name describe = %#v", describe.Fields["Name"])
	}
	if got := describe.Relationships[0].ParentObjects[0]; got != "Account" {
		t.Fatalf("relationship target = %q", got)
	}
	if !describe.Relationships[0].CascadeDelete {
		t.Fatalf("relationship did not carry cascade delete: %#v", describe.Relationships[0])
	}
	if got := describe.Relationships[0].ChildRelationship; got != "Widgets__r" {
		t.Fatalf("child relationship = %q", got)
	}
	if !describe.Relationships[1].Polymorphic {
		t.Fatalf("polymorphic relationship not marked: %#v", describe.Relationships[1])
	}
	if got := describe.Fields["Rating__c"].PicklistValues; len(got) != 1 || got[0].Value != "Hot" || !got[0].Default {
		t.Fatalf("picklist values = %#v", got)
	}
	if got := describe.RecordTypes; len(got) != 1 || got[0].ID != "012000000000001" || got[0].DeveloperName != "Business" || got[0].Name != "Business Widget" || !got[0].Default {
		t.Fatalf("record types = %#v", got)
	}
	definition := ToObjectDefinition(describe)
	if definition.Fields["Account__c"].Type != storage.FieldReference {
		t.Fatalf("definition = %#v", definition)
	}
	if got := definition.Fields["Account__c"].Label; got != "Account__c" {
		t.Fatalf("definition field label = %q", got)
	}
	if got := definition.Fields["Rating__c"].PicklistValues; len(got) != 1 || got[0].Value != "Hot" {
		t.Fatalf("definition picklist values = %#v", got)
	}
	if got := definition.RecordTypes; len(got) != 1 || got[0].ID != "012000000000001" || got[0].DeveloperName != "Business" {
		t.Fatalf("definition record types = %#v", got)
	}
}

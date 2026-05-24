package sobject

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
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
	if value, ok := account.Get("name"); !ok || value.String != "Acme" {
		t.Fatalf("lowercase name = %#v ok=%t", value, ok)
	}
	if value, ok := account.Get("description"); !ok || value.Kind != storage.ValueNull {
		t.Fatalf("lowercase description = %#v ok=%t", value, ok)
	}
	account.Put("name", storage.StringValue("Changed"))
	if _, ok := account.Fields["name"]; ok {
		t.Fatalf("Put created duplicate lowercase field: %#v", account.Fields)
	}
	if value, ok := account.Get("Name"); !ok || value.String != "Changed" {
		t.Fatalf("Name after lowercase Put = %#v ok=%t", value, ok)
	}
	record := account.ToRecord()
	roundTrip := FromRecord(record)
	roundTrip.Put("Name", storage.StringValue("RoundTrip"))
	if original, _ := account.Get("Name"); original.String != "Changed" {
		t.Fatalf("original changed: %#v", original)
	}
}

func TestBuildDescribeRegistry(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
		Name:         "Widget__c",
		Label:        "Widget",
		SharingModel: "ReadWrite",
		NameField: schema.NameField{
			Type:          "AutoNumber",
			Label:         "Widget Number",
			DisplayFormat: "Widget {0000}",
		},
		Fields: []schema.Field{
			{Name: "Account__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Account__r", ChildRelationshipName: "Widgets__r", DeleteConstraint: "Cascade"},
			{Name: "Who__c", Type: "Lookup", ReferenceTo: []string{"Account", "Contact"}, RelationshipName: "Who__r", ChildRelationshipName: "WhoWidgets__r"},
			{Name: "ParentAccount__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Affiliates"},
			{Name: "Master__c", Type: "MasterDetail", ReferenceTo: []string{"Account"}, RelationshipName: "Master__r", ChildRelationshipName: "MasterWidgets__r"},
			{Name: "Rating__c", Type: "Picklist", PicklistValues: []schema.PicklistValue{{FullName: "Hot", Label: "Hot", Default: true, Active: true}}},
			{Name: "PrimaryLocation__c", Type: "Location"},
			{Name: "Notes__c", Type: "LongTextArea"},
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
	if describe.SharingModel != "ReadWrite" {
		t.Fatalf("sharing model = %q", describe.SharingModel)
	}
	if describe.Fields["Name"].Type != storage.FieldString || !describe.Fields["Name"].Required || !describe.Fields["Name"].AutoNumber || describe.Fields["Name"].DisplayFormat != "Widget {0000}" {
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
	if got := describe.Relationships[2].ParentRelationship; got != "ParentAccount__r" {
		t.Fatalf("parent relationship = %q", got)
	}
	if got := describe.Relationships[2].ChildRelationship; got != "Affiliates__r" {
		t.Fatalf("metadata relationship child name = %q", got)
	}
	if !describe.Relationships[3].CascadeDelete {
		t.Fatalf("master-detail relationship did not cascade delete: %#v", describe.Relationships[3])
	}
	if got := describe.Fields["Rating__c"].PicklistValues; len(got) != 1 || got[0].Value != "Hot" || !got[0].Default {
		t.Fatalf("picklist values = %#v", got)
	}
	if got := describe.RecordTypes; len(got) != 1 || got[0].ID != "012000000000001" || got[0].DeveloperName != "Business" || got[0].Name != "Business Widget" || !got[0].Default {
		t.Fatalf("record types = %#v", got)
	}
	if got := describe.Fields["RecordTypeId"]; got.Type != storage.FieldReference || len(got.ReferenceTo) != 1 || got.ReferenceTo[0] != "RecordType" {
		t.Fatalf("RecordTypeId describe = %#v", got)
	}
	definition := ToObjectDefinition(describe)
	if definition.Fields["Account__c"].Type != storage.FieldReference {
		t.Fatalf("definition = %#v", definition)
	}
	if definition.SharingModel != "ReadWrite" {
		t.Fatalf("definition sharing model = %q", definition.SharingModel)
	}
	if got := definition.Fields["Account__c"].ChildRelationshipName; got != "Widgets__r" {
		t.Fatalf("definition child relationship name = %q", got)
	}
	if got := definition.Fields["RecordTypeId"]; got.Type != storage.FieldReference || len(got.ReferenceTo) != 1 || got.ReferenceTo[0] != "RecordType" {
		t.Fatalf("RecordTypeId definition = %#v", got)
	}
	if got := definition.Fields["Name"]; !got.AutoNumber || got.DisplayFormat != "Widget {0000}" {
		t.Fatalf("Name definition = %#v", got)
	}
	if got := definition.Fields["Account__c"].Label; got != "Account__c" {
		t.Fatalf("definition field label = %q", got)
	}
	if got := definition.Fields["Rating__c"].PicklistValues; len(got) != 1 || got[0].Value != "Hot" {
		t.Fatalf("definition picklist values = %#v", got)
	}
	if got := definition.Fields["PrimaryLocation__c"]; got.Type != storage.FieldLocation {
		t.Fatalf("location definition = %#v", got)
	}
	if got := definition.Fields["Notes__c"]; got.Type != storage.FieldString || got.DisplayType != "TEXTAREA" {
		t.Fatalf("textarea definition = %#v", got)
	}
	if got := definition.RecordTypes; len(got) != 1 || got[0].ID != "012000000000001" || got[0].DeveloperName != "Business" {
		t.Fatalf("definition record types = %#v", got)
	}
}

func TestBuildDescribeRegistryMergesDuplicateObjects(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{
		{
			Name: "znu__Order__c",
			Fields: []schema.Field{
				{Name: "znu__Cart__c", Type: "Lookup", ReferenceTo: []string{"znu__Cart__c"}, RelationshipName: "znu__Cart__r"},
			},
		},
		{
			Name: "znu__Order__c",
			Fields: []schema.Field{
				{Name: "znu__Entity__c", Type: "Lookup", ReferenceTo: []string{"znu__Entity__c"}, RelationshipName: "znu__Entity__r"},
			},
		},
	}})

	describe, err := registry.Describe("znu__Order__c")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := describe.Fields["znu__Cart__c"]; !ok {
		t.Fatalf("missing root field: %#v", describe.Fields)
	}
	entity := describe.Fields["znu__Entity__c"]
	if entity.RelationshipName != "znu__Entity__r" || len(entity.ReferenceTo) != 1 || entity.ReferenceTo[0] != "znu__Entity__c" {
		t.Fatalf("entity field = %#v", entity)
	}
	found := false
	for _, relation := range describe.Relationships {
		if relation.Field == "znu__Entity__c" && relation.ParentRelationship == "znu__Entity__r" {
			found = true
		}
	}
	if !found {
		t.Fatalf("relationships = %#v", describe.Relationships)
	}
}

func TestDescribeDefinitionConversionsPreserveFieldDescribeMetadata(t *testing.T) {
	describe := DescribeSObjectResult{
		Name: "Account",
		Fields: map[string]DescribeFieldResult{
			"ExternalLatitude__c": {
				Name:                "ExternalLatitude__c",
				Type:                storage.FieldDecimal,
				DisplayType:         "DOUBLE",
				Label:               "External Latitude",
				Length:              18,
				Precision:           9,
				Scale:               6,
				CompoundFieldName:   "ExternalLocation__c",
				Nillable:            storage.BoolFlag(false),
				DefaultedOnCreate:   storage.BoolFlag(false),
				Accessible:          storage.BoolFlag(true),
				Createable:          storage.BoolFlag(false),
				Updateable:          storage.BoolFlag(false),
				Filterable:          storage.BoolFlag(true),
				Groupable:           storage.BoolFlag(false),
				Sortable:            storage.BoolFlag(true),
				Aggregatable:        storage.BoolFlag(true),
				Permissionable:      storage.BoolFlag(true),
				DeprecatedAndHidden: storage.BoolFlag(false),
				CaseSensitive:       true,
			},
		},
	}

	definition := ToObjectDefinition(describe)
	field := definition.Fields["ExternalLatitude__c"]
	if field.Length != 18 || field.Precision != 9 || field.Scale != 6 || field.CompoundFieldName != "ExternalLocation__c" {
		t.Fatalf("definition field metadata = %#v", field)
	}
	if storage.FieldFlagValue(field.Createable, true) || storage.FieldFlagValue(field.Updateable, true) || !storage.FieldFlagValue(field.Accessible, false) {
		t.Fatalf("definition field flags = %#v", field)
	}
	if !field.CaseSensitive {
		t.Fatalf("definition case sensitive not preserved: %#v", field)
	}

	roundTrip := FromObjectDefinition(definition)
	roundTripField := roundTrip.Fields["ExternalLatitude__c"]
	if roundTripField.Length != 18 || roundTripField.Precision != 9 || roundTripField.Scale != 6 || roundTripField.CompoundFieldName != "ExternalLocation__c" {
		t.Fatalf("round-trip field metadata = %#v", roundTripField)
	}
	if storage.FieldFlagValue(roundTripField.Createable, true) || storage.FieldFlagValue(roundTripField.Updateable, true) || !storage.FieldFlagValue(roundTripField.Accessible, false) {
		t.Fatalf("round-trip field flags = %#v", roundTripField)
	}
	if !roundTripField.CaseSensitive {
		t.Fatalf("round-trip case sensitive not preserved: %#v", roundTripField)
	}
}

func TestFromObjectDefinitionPreservesStandardOverlayDescribeShape(t *testing.T) {
	definition, ok := storage.StandardObjectDefinition("AIInsightAction")
	if !ok {
		t.Fatal("AIInsightAction should be known from the standard SObject overlay")
	}

	describe := FromObjectDefinition(definition)

	if describe.Label != "AIInsightAction" || describe.PluralLabel != "AIInsightActions" {
		t.Fatalf("describe labels = %q, %q", describe.Label, describe.PluralLabel)
	}
	field := describe.Fields["AiRecordInsightId"]
	if field.Type != storage.FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "AIRecordInsight" {
		t.Fatalf("AiRecordInsightId reference = %#v", field)
	}
	if field.RelationshipName != "AiRecordInsight" || field.ChildRelationshipName != "AIInsightActions" {
		t.Fatalf("AiRecordInsightId relationships = %#v", field)
	}
}

func TestFromObjectDefinitionPreservesStandardOverlayPolymorphicBreadth(t *testing.T) {
	definition, ok := storage.StandardObjectDefinition("Task")
	if !ok {
		t.Fatal("Task should be known")
	}

	describe := FromObjectDefinition(definition)

	field := describe.Fields["WhatId"]
	for _, target := range []string{"Account", "Opportunity", "WorkOrder"} {
		if !containsTestString(field.ReferenceTo, target) {
			t.Fatalf("Task.WhatId ReferenceTo missing %s: %#v", target, field.ReferenceTo)
		}
	}
	if !hasDescribeRelationship(describe.Relationships, "WhatId", "Opportunity", "What", "Tasks", true) {
		t.Fatalf("Task.WhatId relationships missing Opportunity.Tasks: %#v", describe.Relationships)
	}
}

func TestBuildDescribeRegistryDerivesCustomMetadataRelationship(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
		Name: "Binding__mdt",
		Fields: []schema.Field{
			{Name: "Target__c", Type: "MetadataRelationship", ReferenceTo: []string{"Target__mdt"}},
		},
	}}})

	describe, err := registry.Describe("Binding__mdt")
	if err != nil {
		t.Fatal(err)
	}
	field := describe.Fields["Target__c"]
	if field.Type != storage.FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Target__mdt" {
		t.Fatalf("field = %#v", field)
	}
	if !hasDescribeRelationship(describe.Relationships, "Target__c", "Target__mdt", "Target__r", "", false) {
		t.Fatalf("relationships = %#v", describe.Relationships)
	}
	definition := ToObjectDefinition(describe)
	if !hasDescribeRelationship(definition.Relations, "Target__c", "Target__mdt", "Target__r", "", false) {
		t.Fatalf("definition relationships = %#v", definition.Relations)
	}
}

func containsTestString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasDescribeRelationship(relations []storage.Relationship, fieldName, parentObject, parentRelationship, childRelationship string, polymorphic bool) bool {
	for _, relation := range relations {
		if relation.Field != fieldName || relation.ParentRelationship != parentRelationship || relation.ChildRelationship != childRelationship || relation.Polymorphic != polymorphic {
			continue
		}
		for _, candidate := range relation.ParentObjects {
			if candidate == parentObject {
				return true
			}
		}
	}
	return false
}

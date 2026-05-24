package orgdescribe

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCatalogToObjectDefinitionsStitchesChildRelationships(t *testing.T) {
	catalog := Catalog{Objects: []SObject{
		{
			Name:        "Account",
			Label:       "Account",
			LabelPlural: "Accounts",
			KeyPrefix:   "001",
			Fields: []Field{
				{Name: "Id", Label: "Account ID", Type: "id", Nillable: false},
				{Name: "Name", Label: "Account Name", Type: "string", Nillable: false, Createable: true, Updateable: true},
			},
			ChildRelationships: []ChildRelationship{{
				ChildSObject:     "Contact",
				Field:            "AccountId",
				RelationshipName: "Contacts",
				CascadeDelete:    true,
			}},
			RecordTypeInfos: []RecordTypeInfo{{
				Name:                     "Master",
				RecordTypeID:             "012000000000000AAA",
				Available:                true,
				DefaultRecordTypeMapping: true,
				Master:                   true,
			}},
		},
		{
			Name:        "Contact",
			Label:       "Contact",
			LabelPlural: "Contacts",
			KeyPrefix:   "003",
			Fields: []Field{
				{Name: "Id", Label: "Contact ID", Type: "id", Nillable: false},
				{Name: "AccountId", Label: "Account ID", Type: "reference", Nillable: true, Createable: true, Updateable: true, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
		},
	}}

	definitions := catalog.ToObjectDefinitions()
	account := definitions["Account"]
	if account.APIName != "Account" || account.KeyPrefix != "001" {
		t.Fatalf("account definition = %#v", account)
	}
	if field := account.Fields["Name"]; field.Type != storage.FieldString || !field.Required {
		t.Fatalf("Account.Name field = %#v", field)
	}
	if field := account.Fields["Id"]; field.Type != storage.FieldID || field.Required {
		t.Fatalf("Account.Id field = %#v", field)
	}
	if len(account.RecordTypes) != 1 || account.RecordTypes[0].DeveloperName != "Master" || account.RecordTypes[0].ID != "012000000000000AAA" || !account.RecordTypes[0].Default {
		t.Fatalf("Account record types = %#v", account.RecordTypes)
	}

	contact := definitions["Contact"]
	field := contact.Fields["AccountId"]
	if field.Type != storage.FieldReference || field.RelationshipName != "Account" || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Account" {
		t.Fatalf("Contact.AccountId field = %#v", field)
	}
	if len(contact.Relations) != 1 {
		t.Fatalf("Contact relationships = %#v", contact.Relations)
	}
	relation := contact.Relations[0]
	if relation.Field != "AccountId" || relation.ParentRelationship != "Account" || relation.ChildRelationship != "Contacts" || !relation.CascadeDelete {
		t.Fatalf("Contact.Account relationship = %#v", relation)
	}
}

func TestSObjectToSchemaObjectMapsDescribeShape(t *testing.T) {
	object := SObject{
		Name:        "Opportunity",
		Label:       "Opportunity",
		LabelPlural: "Opportunities",
		Fields: []Field{
			{
				Name:       "StageName",
				Label:      "Stage",
				Type:       "picklist",
				Nillable:   false,
				Createable: true,
				Updateable: true,
				PicklistValues: []PicklistValue{
					{Value: "Prospecting", Label: "Prospecting", Active: true, Default: true},
					{Value: "Closed Won", Label: "Closed Won", Active: true},
				},
			},
			{
				Name:             "OwnerId",
				Label:            "Owner ID",
				Type:             "reference",
				Nillable:         false,
				Createable:       true,
				Updateable:       true,
				ReferenceTo:      []string{"User", "Group"},
				RelationshipName: "Owner",
			},
			{
				Name:       "AmountFormula__c",
				Label:      "Amount Formula",
				Type:       "currency",
				Calculated: true,
				Formula:    "Amount * 2",
			},
		},
		RecordTypeInfos: []RecordTypeInfo{{
			Name:                     "Enterprise",
			DeveloperName:            "Enterprise",
			Available:                true,
			DefaultRecordTypeMapping: true,
		}},
	}

	schemaObject := object.ToSchemaObject(map[string]string{"OwnerId": "OwnedOpportunities"})
	if schemaObject.Name != "Opportunity" || schemaObject.PluralLabel != "Opportunities" {
		t.Fatalf("schema object = %#v", schemaObject)
	}
	fields := map[string]FieldLike{}
	for _, field := range schemaObject.Fields {
		fields[field.Name] = FieldLike{
			Type:                  field.Type,
			RelationshipName:      field.RelationshipName,
			ChildRelationshipName: field.ChildRelationshipName,
			Required:              field.Required,
			Picklists:             len(field.PicklistValues),
			Formula:               field.Formula,
		}
	}
	if got := fields["StageName"]; got.Type != "Picklist" || !got.Required || got.Picklists != 2 {
		t.Fatalf("StageName mapping = %#v", got)
	}
	if got := fields["OwnerId"]; got.Type != "Lookup" || got.RelationshipName != "Owner" || got.ChildRelationshipName != "OwnedOpportunities" {
		t.Fatalf("OwnerId mapping = %#v", got)
	}
	if got := fields["AmountFormula__c"]; got.Type != "Currency" || got.Formula != "Amount * 2" {
		t.Fatalf("formula field mapping = %#v", got)
	}
	if len(schemaObject.RecordTypes) != 1 || schemaObject.RecordTypes[0].DeveloperName != "Enterprise" || !schemaObject.RecordTypes[0].Default {
		t.Fatalf("record types = %#v", schemaObject.RecordTypes)
	}
}

func TestDescribeFieldMappingPreservesRuntimeShape(t *testing.T) {
	field := Field{
		Name:             "WhatId",
		Label:            "Related To",
		Type:             "reference",
		Nillable:         true,
		Createable:       true,
		Updateable:       true,
		ReferenceTo:      []string{"Account", "Opportunity"},
		RelationshipName: "What",
	}

	describe := field.ToDescribeFieldResult()
	if describe.Type != storage.FieldReference || describe.DisplayType != "REFERENCE" {
		t.Fatalf("describe field type = %#v", describe)
	}
	if describe.RelationshipName != "What" || len(describe.ReferenceTo) != 2 {
		t.Fatalf("describe relationship = %#v", describe)
	}
}

type FieldLike struct {
	Type                  string
	RelationshipName      string
	ChildRelationshipName string
	Required              bool
	Picklists             int
	Formula               string
}

package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A change-event trigger on a custom child must not make its event object a
// second owner of the base child's relationship name on the parent.
func TestChangeEventChildKeepsParentChildRelationshipAnonymousAndIsTestRoutes(t *testing.T) {
	const body = "Account parent = new Account(Name = 'x'); List<Child__c> kids = parent.Children; " +
		"Account queried = [SELECT Id, (SELECT Id FROM Children) FROM Account LIMIT 1]; Child__c first = queried.Children[0];"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				objects := []schema.Object{{Name: "Child__c", Fields: []schema.Field{
					{Name: "Parent__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r", ChildRelationshipName: "Children"},
					{Name: "Marker__c", Type: "Text"},
				}}}
				root := t.TempDir()
				meta := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
				trigger := filepath.Join(root, "ChildChangeObserver.trigger")
				writeSemaFile(t, trigger, "trigger ChildChangeObserver on Child__ChangeEvent (after insert) { }")
				writeSemaFile(t, trigger+"-meta.xml", `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
				files := []string{trigger}
				if route == "isTest" {
					class := filepath.Join(root, "ChildRoute.cls")
					writeSemaFile(t, class, "@IsTest private class ChildRoute { @IsTest static void observe() { "+body+" } }")
					writeSemaFile(t, class+"-meta.xml", meta)
					files = append(files, class)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, schema.Schema{})
				index.Objects = objects
				if index.HasErrors() {
					t.Fatalf("index: %v", index.Diagnostics)
				}
				var result Result
				if route == "anonymous" {
					if result = Analyze(index); result.HasErrors() {
						t.Fatalf("trigger project: %v", result.Diagnostics)
					}
					result = AnalyzeAnonymous(index, body, api)
				} else {
					result = Analyze(index)
				}
				if result.HasErrors() {
					t.Fatalf("parent child relationship resolved away from Child__c: %v", result.Diagnostics)
				}
			})
		}
	}
}

// Native describe rows (change_event_describe, APIs 62.0 and 67.0) list every
// custom field of the base on its change event: lookup, number, formula,
// picklist, checkbox and text. The base lookup's child relationship stays on
// the base object.
func TestChangeEventCustomFieldsAndParentRelationshipAnonymousAndIsTestRoutes(t *testing.T) {
	const body = "Parcel__ChangeEvent event = new Parcel__ChangeEvent(); Object values = new List<Object>{ event.Acct__c, event.Weight__c, event.Fx__c, event.Kind__c, event.Flag__c, event.Note__c, event.ChangeEventHeader }; " +
		"Account parent = new Account(Name = 'x'); List<Parcel__c> parcels = parent.Parcels__r;"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				objects := []schema.Object{{Name: "Parcel__c", Fields: []schema.Field{
					{Name: "Acct__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Acct__r", ChildRelationshipName: "Parcels"},
					{Name: "Weight__c", Type: "Number", Precision: 10, Scale: 2},
					{Name: "Fx__c", Type: "Number", Formula: "Weight__c * 2", Precision: 18, Scale: 2},
					{Name: "Kind__c", Type: "Picklist"},
					{Name: "Flag__c", Type: "Checkbox"},
					{Name: "Note__c", Type: "Text", Length: 50},
				}}}
				root := t.TempDir()
				trigger := filepath.Join(root, "ParcelChangeObserver.trigger")
				writeSemaFile(t, trigger, "trigger ParcelChangeObserver on Parcel__ChangeEvent (after insert) { }")
				writeSemaFile(t, trigger+"-meta.xml", `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
				files := []string{trigger}
				if route == "isTest" {
					class := filepath.Join(root, "ParcelRoute.cls")
					writeSemaFile(t, class, "@IsTest private class ParcelRoute { @IsTest static void observe() { "+body+" } }")
					writeSemaFile(t, class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					files = append(files, class)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, schema.Schema{})
				index.Objects = objects
				if index.HasErrors() {
					t.Fatalf("index: %v", index.Diagnostics)
				}
				result := Analyze(index)
				if route == "anonymous" && !result.HasErrors() {
					result = AnalyzeAnonymous(index, body, api)
				}
				if result.HasErrors() {
					t.Fatalf("change event custom fields or parent relationship rejected: %v", result.Diagnostics)
				}
			})
		}
	}
}

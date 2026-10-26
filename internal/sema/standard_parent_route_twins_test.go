package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardParentPartialFieldAnonymousNamedAndIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "isTest"} {
			for _, tc := range []struct{ name, body string }{
				{"parent_member", "ParentLink__c row = new ParentLink__c(); Object note = row.Contact__r.LocalNote__c;"},
				{"direct_member", "Contact parent = new Contact(); Object note = parent.LocalNote__c;"},
				{"soql_parent_member", "ParentLink__c row = [SELECT Contact__r.LocalNote__c FROM ParentLink__c LIMIT 1]; Object note = row.Contact__r.LocalNote__c;"},
				{"standard_field_and_child_alias", "Contact parent = new Contact(LastName='Parent'); String name = parent.LastName; List<ParentLink__c> links = parent.ParentLinks__r;"},
			} {
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					objects := []schema.Object{
						{Name: "ParentLink__c", Fields: []schema.Field{{Name: "Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: "Contact__r", ChildRelationshipName: "ParentLinks__r"}}},
						// The local extension omits a field supplied by another metadata source.
						// The native project declares LocalNote__c; see standard_parent_routes.
						{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "Marker__c", Type: "Text"}}},
					}
					index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}, Objects: objects}
					var result Result
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, tc.body, api)
					} else {
						root := t.TempDir()
						path := filepath.Join(root, "ParentRoute.cls")
						source := "public class ParentRoute { public static void observe() { " + tc.body + " } }"
						if route == "isTest" {
							source = "@IsTest private class ParentRoute { @IsTest static void observe() { " + tc.body + " } }"
						}
						writeSemaFile(t, path, source)
						writeSemaFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
						index = typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
						index.Objects = objects
						if index.HasErrors() {
							t.Fatalf("index: %v", index.Diagnostics)
						}
						result = Analyze(index)
					}
					if result.HasErrors() {
						t.Fatalf("partial standard parent lost field fallback: %v", result.Diagnostics)
					}
				})
			}
		}
	}
}

// The native dynamic-query rejection in standard_parent_routes pins the
// missing leaf; complete metadata must retain its static query diagnostic.
func TestStandardParentCompleteMetadataRejectsMissingLeafAnonymousRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}, Objects: []schema.Object{
				{Name: "ParentLink__c", Fields: []schema.Field{{Name: "Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: "Contact__r", ChildRelationshipName: "ParentLinks__r"}}},
				{Name: "Contact", Fields: []schema.Field{{Name: "LocalNote__c", Type: "Text"}}},
			}}
			result := AnalyzeAnonymous(index, "List<ParentLink__c> rows = [SELECT Contact__r.AbsentProbeField FROM ParentLink__c];", api)
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_FIELD") {
				t.Fatalf("complete parent accepted missing leaf: %v", result.Diagnostics)
			}
		})
	}
}

// SOSL retains the field checks from before the SOQL-only partial-parent fallback.
func TestStandardParentSOSLPreservationAnonymousNamedAndIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "isTest"} {
			for _, tc := range []struct {
				leaf   string
				reject bool
			}{{"LocalNote__c", true}, {"LastName", false}, {"Marker__c", false}} {
				t.Run(api+"/"+route+"/"+tc.leaf, func(t *testing.T) {
					objects := []schema.Object{
						{Name: "ParentLink__c", Fields: []schema.Field{{Name: "Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: "Contact__r"}}},
						{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "Marker__c", Type: "Text"}}},
					}
					body := "List<List<SObject>> rows = [FIND 'Parent' RETURNING ParentLink__c(Contact__r." + tc.leaf + ")];"
					index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}, Objects: objects}
					var result Result
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, body, api)
					} else {
						root := t.TempDir()
						path := filepath.Join(root, "ParentSearchRoute.cls")
						source := "public class ParentSearchRoute { public static void observe() { " + body + " } }"
						if route == "isTest" {
							source = "@IsTest private class ParentSearchRoute { @IsTest static void observe() { " + body + " } }"
						}
						writeSemaFile(t, path, source)
						writeSemaFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
						index = typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
						index.Objects = objects
						if index.HasErrors() {
							t.Fatal(index.Diagnostics)
						}
						result = Analyze(index)
					}
					if tc.reject {
						if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA_SOSL_FIELD" {
							t.Fatalf("SOSL omitted parent leaf must retain its diagnostic: %v", result.Diagnostics)
						}
					} else if result.HasErrors() {
						t.Fatalf("SOSL known parent leaf rejected: %v", result.Diagnostics)
					}
				})
			}
		}
	}
}

func TestStandardParentSOSLCompleteMetadataRejectsMissingLeafAnonymousRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}, Objects: []schema.Object{
				{Name: "ParentLink__c", Fields: []schema.Field{{Name: "Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: "Contact__r"}}},
				{Name: "Contact", Fields: []schema.Field{{Name: "Marker__c", Type: "Text"}}},
			}}
			result := AnalyzeAnonymous(index, "List<List<SObject>> rows = [FIND 'Parent' RETURNING ParentLink__c(Contact__r.LocalNote__c)];", api)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA_SOSL_FIELD" {
				t.Fatalf("complete parent must retain its SOSL missing-leaf diagnostic: %v", result.Diagnostics)
			}
		})
	}
}

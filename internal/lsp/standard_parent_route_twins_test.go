package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardParentPartialFieldNamedAndIsTestLSPRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"named", "isTest"} {
			for _, shape := range []string{"member", "soql"} {
				t.Run(api+"/"+route+"/"+shape, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "ParentRoute.cls")
					body := "ParentLink__c row = new ParentLink__c(); Object note = row.Contact__r.LocalNote__c;"
					if shape == "soql" {
						body = "ParentLink__c row = [SELECT Contact__r.LocalNote__c FROM ParentLink__c LIMIT 1]; Object note = row.Contact__r.LocalNote__c;"
					}
					source := "public class ParentRoute { public static void observe() { " + body + " } }"
					if route == "isTest" {
						source = "@IsTest private class ParentRoute { @IsTest static void observe() { " + body + " } }"
					}
					if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
						t.Fatal(err)
					}
					metadata := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
					if err := os.WriteFile(path+"-meta.xml", []byte(metadata), 0o644); err != nil {
						t.Fatal(err)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					index.Objects = []schema.Object{
						{Name: "ParentLink__c", Fields: []schema.Field{{Name: "Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: "Contact__r", ChildRelationshipName: "ParentLinks__r"}}},
						{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "Marker__c", Type: "Text"}}},
					}
					handler := NewHandler(index)
					t.Cleanup(func() { handler.parser.Close() })
					uri := uriFromPath(path)
					notifications := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: source}})
					if got := diagnosticsCount(t, notifications); got != 0 {
						t.Fatalf("native-backed parent route has %d diagnostics: %+v", got, notifications)
					}
					notifications, err := handler.DidChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2}, ContentChanges: []TextDocumentContentChangeEvent{{Text: "public class ParentRoute {"}}})
					if err != nil {
						t.Fatal(err)
					}
					if diagnosticsCount(t, notifications) == 0 {
						t.Fatal("malformed overlay produced no diagnostic")
					}
				})
			}
		}
	}
}

func TestStandardParentSOSLPreservationNamedAndIsTestLSPRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"named", "isTest"} {
			for _, leaf := range []string{"LocalNote__c", "LastName", "Marker__c"} {
				t.Run(api+"/"+route+"/"+leaf, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "ParentSearchRoute.cls")
					body := "List<List<SObject>> rows = [FIND 'Parent' RETURNING ParentLink__c(Contact__r." + leaf + ")];"
					source := "public class ParentSearchRoute { public static void observe() { " + body + " } }"
					if route == "isTest" {
						source = "@IsTest private class ParentSearchRoute { @IsTest static void observe() { " + body + " } }"
					}
					if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
						t.Fatal(err)
					}
					metadata := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
					if err := os.WriteFile(path+"-meta.xml", []byte(metadata), 0o644); err != nil {
						t.Fatal(err)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					index.Objects = []schema.Object{
						{Name: "ParentLink__c", Fields: []schema.Field{{Name: "Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: "Contact__r"}}},
						{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "Marker__c", Type: "Text"}}},
					}
					handler := NewHandler(index)
					t.Cleanup(func() { handler.parser.Close() })
					uri := uriFromPath(path)
					check := func(notifications []Notification) {
						t.Helper()
						want := 0
						if leaf == "LocalNote__c" {
							want = 1
						}
						if got := diagnosticsCount(t, notifications); got != want {
							t.Fatalf("SOSL LSP diagnostics = %d, want %d: %+v", got, want, notifications)
						}
						if want == 1 && notifications[0].Params.(PublishDiagnosticsParams).Diagnostics[0].Code != "GLADESEMA_SOSL_FIELD" {
							t.Fatalf("unrelated diagnostic: %+v", notifications)
						}
					}
					check(handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: source}}))
					// Edits are parsed against the handler's fixed semantic index snapshot.
					notifications, err := handler.DidChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2}, ContentChanges: []TextDocumentContentChangeEvent{{Text: source + "\n"}}})
					if err != nil {
						t.Fatal(err)
					}
					check(notifications)
				})
			}
		}
	}
}

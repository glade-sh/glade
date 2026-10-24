package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardParentSOQLValidEditNamedAndIsTestLSPRoutes(t *testing.T) {
	for _, namespace := range []string{"gladens001", "gladens003"} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"named", "isTest"} {
				for _, leaf := range []string{"LocalNote__c", "gladens001__ParentRouteValue__c"} {
					t.Run(namespace+"/"+api+"/"+route+"/"+leaf, func(t *testing.T) {
						root := t.TempDir()
						path := filepath.Join(root, "ParentEditRoute.cls")
						body := "ParentLink__c row = [SELECT Contact__r." + leaf + " FROM ParentLink__c LIMIT 1]; Object note = row.Contact__r." + leaf + ";"
						source := "public class ParentEditRoute { public static void observe() { " + body + " } }"
						if route == "isTest" {
							source = "@IsTest private class ParentEditRoute { @IsTest static void observe() { " + body + " } }"
						}
						if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path+"-meta.xml", []byte(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`), 0o644); err != nil {
							t.Fatal(err)
						}
						index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
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
						// The handler uses a fixed semantic index. Repair a malformed overlay to
						// that indexed source so DidChange must return the SOQL semantic result.
						opened := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: "public class ParentEditRoute {"}})
						if diagnosticsCount(t, opened) == 0 {
							t.Fatal("malformed open produced no diagnostic")
						}
						for _, edit := range []struct {
							version int
							text    string
						}{{2, source}, {3, source + "\n"}} {
							notifications, err := handler.DidChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: edit.version}, ContentChanges: []TextDocumentContentChangeEvent{{Text: edit.text}}})
							if err != nil {
								t.Fatal(err)
							}
							if got := diagnosticsCount(t, notifications); got != 0 {
								t.Fatalf("valid SOQL edit version %d has %d diagnostics: %+v", edit.version, got, notifications)
							}
						}
					})
				}
			}
		}
	}
}

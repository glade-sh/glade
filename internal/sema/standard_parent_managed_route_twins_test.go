package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardParentManagedFieldAnonymousNamedAndIsTestRoutes(t *testing.T) {
	for _, namespace := range []string{"gladens001", "gladens003"} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"anonymous", "named", "isTest"} {
				for _, shape := range []string{"member", "soql"} {
					t.Run(namespace+"/"+api+"/"+route+"/"+shape, func(t *testing.T) {
						objects := []schema.Object{
							{Name: namespace + "__ParentRouteLink__c", Fields: []schema.Field{{Name: namespace + "__Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: namespace + "__Contact__r"}}},
							{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "Marker__c", Type: "Text"}}},
						}
						body := "gladens001__ParentRouteLink__c row = new gladens001__ParentRouteLink__c(); Object note = row.gladens001__Contact__r.gladens001__ParentRouteValue__c;"
						if shape == "soql" {
							body = "gladens001__ParentRouteLink__c row = [SELECT gladens001__Contact__r.gladens001__ParentRouteValue__c FROM gladens001__ParentRouteLink__c LIMIT 1]; Object note = row.gladens001__Contact__r.gladens001__ParentRouteValue__c;"
						}
						body = strings.ReplaceAll(body, "gladens001__ParentRouteLink", namespace+"__ParentRouteLink")
						body = strings.ReplaceAll(body, "gladens001__Contact__", namespace+"__Contact__")
						index := typesys.Index{Project: typesys.ProjectInfo{Namespace: namespace, SourceAPIVersion: api}, Objects: objects}
						var result Result
						if route == "anonymous" {
							result = AnalyzeAnonymous(index, body, api)
						} else {
							root := t.TempDir()
							path := filepath.Join(root, "ManagedParentRoute.cls")
							source := "public class ManagedParentRoute { public static void observe() { " + body + " } }"
							if route == "isTest" {
								source = "@IsTest private class ManagedParentRoute { @IsTest static void observe() { " + body + " } }"
							}
							writeSemaFile(t, path, source)
							writeSemaFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
							index = typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
							index.Objects = objects
							if index.HasErrors() {
								t.Fatal(index.Diagnostics)
							}
							result = Analyze(index)
						}
						if result.HasErrors() {
							t.Fatalf("namespace-qualified partial-parent %s route: %v", shape, result.Diagnostics)
						}
					})
				}
			}
		}
	}
}

package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A mutable host owns its schema containers. Their addresses do not prove
// immutable content. Warming any reader must not change the next generation's
// outcomes, including inherited errors. This is release compatibility, not a
// new native rule: compare the exact result with an unprimed VM on that schema.
func TestChildRelationshipIndexesDiscardMutableGeneration(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"field", "relation", "both"} {
			for _, edit := range []string{"slice", "replace", "remove-add"} {
				for _, boundary := range []string{"same", "copy", "template-fallback", "generation", "metadata"} {
					for _, route := range []string{"JSON", "JSONStrict", "getSObjects", "field", "SOQL"} {
						t.Run(fmt.Sprintf("%s/%s/%s/%s/%s", api, shape, edit, boundary, route), func(t *testing.T) {
							org := storage.NewOrgState()
							for _, name := range []string{"Cedar__c", "Maple__c", "Parcel__c"} {
								org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
							}
							child := org.Objects["Parcel__c"]
							if shape != "relation" {
								child.Definition.Fields["Parent__c"] = storage.Field{APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Cedar__c"}, RelationshipName: "Parent__r", ChildRelationshipName: "Parcels__r"}
							}
							if shape != "field" {
								child.Definition.Relations = []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Cedar__c"}, ParentRelationship: "Parent__r", ChildRelationship: "Parcels__r"}}
							}
							org.Objects["Parcel__c"] = child
							machine := New(nil)
							machine.SetOrg(&org)
							execute := func(vm *VM, source string) string {
								program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
								if err == nil {
									_, err = vm.Execute(program)
								}
								if err != nil {
									return err.Error()
								}
								return "PASS"
							}
							jsonSource := func(parent, method string) string {
								return parent + ` row = (` + parent + `)JSON.` + method + `('{"Parcels__r":{"totalSize":1,"done":true,"records":[{"Name":"owned"}]}}', ` + parent + `.class); System.assertEquals('Parcel__c', String.valueOf(row.Parcels__r[0].getSObjectType()));`
							}
							if got := execute(machine, jsonSource("Cedar__c", "deserialize")); got != "PASS" {
								t.Fatal("warmup:", got)
							}
							child = org.Objects["Parcel__c"]
							if shape != "relation" {
								field := child.Definition.Fields["Parent__c"]
								switch edit {
								case "slice":
									field.ReferenceTo[0] = "Maple__c"
								case "replace":
									field.ReferenceTo = []string{"Maple__c"}
								case "remove-add":
									field.ReferenceTo = []string{"Maple__c"}
									field.APIName = "Other__c"
									field.RelationshipName = "Other__r"
									delete(child.Definition.Fields, "Parent__c")
								}
								child.Definition.Fields[field.APIName] = field
							}
							if shape != "field" {
								switch edit {
								case "slice":
									child.Definition.Relations[0].ParentObjects[0] = "Maple__c"
								case "replace":
									child.Definition.Relations[0].ParentObjects = []string{"Maple__c"}
								case "remove-add":
									child.Definition.Relations[0] = storage.Relationship{Field: "Other__c", ParentObjects: []string{"Maple__c"}, ParentRelationship: "Other__r", ChildRelationship: "Parcels__r"}
								}
							}
							org.Objects["Parcel__c"] = child
							switch boundary {
							case "same":
								machine.SetOrg(&org)
							case "copy":
								copyOrg := org
								machine.SetOrg(&copyOrg)
							case "template-fallback":
								org.ClearRuntimeSchemaStamp()
								machine.SetRuntimeTemplateOrg(&org)
							case "generation":
								org.ClearRuntimeSchemaStamp()
							case "metadata":
								machine.clearMetadataCaches()
							}
							freshOrg := machine.Org.Clone()
							fresh := New(nil)
							fresh.SetOrg(&freshOrg)
							source := ""
							switch route {
							case "JSON":
								source = jsonSource("Maple__c", "deserialize")
							case "JSONStrict":
								source = jsonSource("Maple__c", "deserializeStrict")
							case "getSObjects":
								source = `Maple__c row = new Maple__c(); Object children = row.getSObjects('Parcels__r');`
							case "field":
								source = `Maple__c row = new Maple__c(); Object children = row.Parcels__r;`
							case "SOQL":
								source = `List<Maple__c> rows = [SELECT Id, (SELECT Id FROM Parcels__r) FROM Maple__c];`
							}
							want := execute(fresh, source)
							// Release baseline retains precomputed relation targets after an
							// in-place slice edit without SetOrg. Preserve these inherited errors;
							// this optimization must not broaden host schema behavior.
							if edit == "slice" && (boundary == "generation" || boundary == "metadata") {
								if route == "getSObjects" && shape != "field" {
									want = "SObjectException: Invalid aggregate relationship Parcels__r for Maple__c"
								}
								if shape == "relation" {
									if route == "JSON" {
										want = "JSONException: No field name specified on column for sobject of type Maple__c"
									}
									if route == "JSONStrict" {
										want = "JSONException: No such column 'Parcels__r' on sobject of type Maple__c"
									}
								}
							}
							if got := execute(machine, source); got != want {
								t.Fatalf("warm=%s; fresh=%s", got, want)
							}
						})
					}
				}
			}
		}
	}
}

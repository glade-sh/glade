package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Invalidation must cover positive and negative hits, not only a previously
// unseen relationship key. Compare with a fresh reader of the exact schema.
// These exercise the host's declared invalidation API, not new Apex semantics.
func TestMetadataReadersObserveGenerationChanges(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, frozen := range []bool{false, true} {
			for _, boundary := range []string{"clear", "invalidate", "mutable-definition"} {
				for _, reader := range []string{"global", "object", "field", "relationship", "missing-field"} {
					t.Run(fmt.Sprintf("%s/frozen=%v/%s/%s", api, frozen, boundary, reader), func(t *testing.T) {
						org := describeShareOrg()
						org.Objects["Parcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parcel__c", Fields: map[string]storage.Field{
							"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r", ChildRelationshipName: "Parcels__r"},
						}}}
						machine := New(nil)
						if frozen {
							template := storage.NewRuntimeTemplate(org)
							PrimeRuntimeTemplateSchema(&template)
							org = template.CloneRuntimeOrg()
							machine.PrimeMetadataSchema(&org)
							machine.SetRuntimeTemplateOrg(&org)
						} else {
							machine.SetOrg(&org)
						}
						read := func(vm *VM) string {
							switch reader {
							case "global":
								_, present := vm.schemaGlobalDescribe().Map[mapKey(String("Parcel__c"))]
								return fmt.Sprint(present)
							case "object":
								return vm.describeSObjectValue("Account", vm.Org.Objects["Account"].Definition).Fields["label"].Text
							case "field":
								value, err := vm.describeFieldValue("Account", "Name")
								if err != nil {
									return err.Error()
								}
								return value.Fields["label"].Text
							case "relationship":
								value, ok := vm.jsonSObjectChildRelationshipType("Account", "Parcels__r")
								return fmt.Sprint(value, ok)
							case "missing-field":
								value, err := vm.describeFieldValue("Account", "Fresh__c")
								return fmt.Sprint(value.Fields["label"].Text, err)
							}
							panic("unknown reader")
						}
						before := read(machine)
						// Privatize before editing frozen definitions; no test writes a
						// template's shared maps or a sibling's backing storage.
						state := org.Objects["Account"]
						state.Definition = state.Definition.Clone()
						state.Definition.Label = "Fresh account"
						field := state.Definition.Fields["Name"]
						field.Label = "Fresh name"
						state.Definition.Fields["Name"] = field
						state.Definition.Fields["Fresh__c"] = storage.Field{APIName: "Fresh__c", Type: storage.FieldString, Label: "Fresh field"}
						org.Objects["Account"] = state
						delete(org.Objects, "Parcel__c")
						org.Objects["Carry__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Carry__c", Fields: map[string]storage.Field{}}}
						switch boundary {
						case "clear":
							org.ClearRuntimeSchemaStamp()
						case "invalidate":
							org.InvalidateRuntimeGeneration()
						case "mutable-definition":
							storage.EnsureMutableObjectDefinition(&org, "Account")
						}
						fresh := New(nil)
						freshOrg := org.Clone()
						fresh.SetOrg(&freshOrg)
						want := read(fresh)
						if want == before {
							t.Fatal("mutation did not change the control observation")
						}
						if got := read(machine); got != want {
							t.Fatalf("warm=%q fresh=%q before=%q", got, want, before)
						}
						// The public execution route must use the same generation too.
						program, err := CompileAnonymousWithOptions(`System.assertEquals('Fresh account', Account.SObjectType.getDescribe().getLabel());`, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEnsureObjectOwnsSchemaRepairsAndPreservesNoops(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "Owned__c", Fields: map[string]storage.Field{
		"Name":      {APIName: "Name", Type: storage.FieldString},
		"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
	}, Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Account"}}}}
	for _, repair := range []string{"none", "object", "field", "relation", "records"} {
		t.Run(repair, func(t *testing.T) {
			org := storage.NewOrgState()
			ensureObject(&org, definition.Clone())
			state := org.Objects["Owned__c"]
			switch repair {
			case "object":
				delete(org.Objects, "Owned__c")
			case "field":
				delete(state.Definition.Fields, "Name")
			case "relation":
				state.Definition.Relations = nil
			case "records":
				state.Records = nil
			}
			if repair != "object" {
				org.Objects["Owned__c"] = state
			}
			template := storage.NewRuntimeTemplate(org)
			PrimeRuntimeTemplateSchema(&template)
			writer, sibling := template.CloneRuntimeOrg(), template.CloneRuntimeOrg()
			before := writer.RuntimeGenerationID()
			source := describeShareSource(writer.Objects["Owned__c"].Definition)
			ensureObject(&writer, definition)
			changed := repair == "object" || repair == "field" || repair == "relation"
			if (writer.RuntimeGenerationID() != before) != changed {
				t.Fatalf("repair=%s generation changed=%v", repair, writer.RuntimeGenerationID() != before)
			}
			if !changed && describeShareSource(writer.Objects["Owned__c"].Definition) != source {
				t.Fatal("no schema repair copied a frozen definition")
			}
			if sibling.RuntimeGenerationID() != before {
				t.Fatal("writer invalidated its sibling")
			}
			if repair == "field" {
				if _, ok := sibling.Objects["Owned__c"].Definition.Fields["Name"]; ok {
					t.Fatal("field repair reached sibling")
				}
			}
			if repair == "relation" && len(sibling.Objects["Owned__c"].Definition.Relations) != 0 {
				t.Fatal("relationship repair reached sibling")
			}
		})
	}
}

func TestEnsureObjectCompleteDefinitionDoesNotAllocate(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "Owned__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}
	ensureObject(&org, definition)
	if allocations := testing.AllocsPerRun(20, func() { ensureObject(&org, definition) }); allocations > 0 {
		t.Fatalf("complete ensure allocated %.0f times", allocations)
	}
}

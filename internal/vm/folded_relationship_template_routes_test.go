package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFoldedRelationshipFrozenTemplateAllocationRoutes(t *testing.T) {
	for _, restore := range []bool{false, true} {
		route := "cold"
		if restore {
			route = "serialized_restore"
		}
		t.Run(route, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["Meadow__c"] = storage.ObjectState{Definition: foldedParentDefinition(20)}
			if restore {
				data, err := json.Marshal(org)
				if err != nil {
					t.Fatal(err)
				}
				org = storage.NewOrgState()
				if err := json.Unmarshal(data, &org); err != nil {
					t.Fatal(err)
				}
			}
			template := NewRestoredRuntimeTemplate(org, New(nil))
			for range 2 {
				clone := template.CloneOrg()
				relation := clone.Objects["Meadow__c"].Definition.Relations[0]
				lookup := func() { foldedNameCommonResult = relationshipTargetsObject(relation, "pkg__Missing") }
				if got := testing.AllocsPerRun(100, lookup); got != 0 {
					t.Errorf("frozen template lookup allocations = %.0f, want 0", got)
				}
			}
		})
	}
}

func TestFoldedRelationshipTemplatePrimingPreservesGenerationRoute(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Meadow__c"] = storage.ObjectState{Definition: foldedParentDefinition(20)}
	template := storage.NewRuntimeTemplate(org)
	PrimeRuntimeTemplateSchema(&template)
	generation := template.Org.RuntimeGenerationID()
	stamp := template.Org.RuntimeSchemaStamp
	PrimeRuntimeTemplateSchema(&template)
	if template.Org.RuntimeGenerationID() != generation || template.Org.RuntimeSchemaStamp != stamp {
		t.Fatal("repeated derived-key preparation invalidated the schema generation")
	}
}

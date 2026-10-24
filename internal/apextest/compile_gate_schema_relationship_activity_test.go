package apextest

import (
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

func TestSchemaReferenceGateRelationshipInheritedActivityRoutes(t *testing.T) {
	for _, object := range []string{"Task", "Event"} {
		t.Run(object, func(t *testing.T) {
			metadata := schemaRelationshipGateMetadata("Account")
			metadata.Objects = append(metadata.Objects, schema.Object{Name: "Activity", Fields: []schema.Field{{Name: "Client__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Activities"}}})
			index := typesys.Index{Objects: metadata.Objects}
			source := `public class Probe { Object items = [SELECT Client__r.Destination__c FROM ` + object + `]; }`
			if !compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), "Account") {
				t.Fatal("inherited Activity lookup target was lost")
			}
		})
	}
}

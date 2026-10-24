package apextest

import (
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

func TestSchemaReferenceGateRelationshipMergedObjectExtensions(t *testing.T) {
	relationship := schema.Object{Name: "Contact", Fields: []schema.Field{{Name: "Client__c", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Clients"}}}
	extension := schema.Object{Name: "Contact", Fields: []schema.Field{{Name: "Label__c", Type: "Text"}}}
	for _, objects := range [][]schema.Object{{relationship, extension}, {extension, relationship}} {
		index := typesys.Index{Objects: objects}
		source := `public class Probe { Object items = [SELECT Client__r.Destination__c FROM Contact]; }`
		if !compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), "Account") {
			t.Fatal("object extension overwrote a relationship field")
		}
	}
}

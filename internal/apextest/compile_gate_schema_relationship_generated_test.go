package apextest

import (
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

func TestSchemaReferenceGateRelationshipGeneratedNamespaceRoutes(t *testing.T) {
	for _, object := range []string{"Parcel__History", "pkg__Parcel__History", "Parcel__Share", "pkg__Parcel__Share"} {
		t.Run(object, func(t *testing.T) {
			index := typesys.Index{Project: typesys.ProjectInfo{Namespace: "pkg"}, Objects: []schema.Object{{Name: "pkg__Parcel__c", EnableHistory: true, EnableSharing: true, SharingModel: "ReadWrite", Fields: []schema.Field{{Name: "Destination__c", Type: "Lookup", ReferenceTo: []string{"UnavailableDestination"}}}}}}
			source := `public class Probe { Object records = [SELECT Parent.Destination__c FROM ` + object + `]; }`
			if !compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), "pkg__Parcel__c") {
				t.Fatal("generated object lost its namespaced parent target")
			}
		})
	}
}

package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Matching native Task/Event TYPEOF rows: ../sema/testdata/schema_relationship_gate/
// org.tsv:27-28,75-76,117-118,172-173,220-221,262-263 (API 62/67).
func TestSchemaReferenceGateRelationshipTypeofCustomActivityRoutes(t *testing.T) {
	for _, object := range []string{"Task", "Event"} {
		t.Run(object, func(t *testing.T) {
			metadata := schemaRelationshipGateMetadata("Parcel__c")
			metadata.Objects[0].EnableActivities = true
			index := typesys.Index{Objects: metadata.Objects}
			source := `public class Probe { Object items = [SELECT TYPEOF What WHEN Parcel__c THEN Destination__c END FROM ` + object + `]; }`
			if !compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), "Parcel__c") {
				t.Fatal("explicit TYPEOF custom target lost its metadata ownership")
			}
		})
	}
}

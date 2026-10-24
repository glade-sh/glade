package apextest

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaReferenceGateRelationshipRepeatedLookupPreservesIndex(t *testing.T) {
	index := typesys.Index{Objects: schemaRelationshipGateMetadata("User").Objects}
	before, err := json.Marshal(index.Objects)
	if err != nil {
		t.Fatal(err)
	}
	units := newCompileGateUnits(index)
	for range 2 {
		source := `public class Probe { Object items = [SELECT Owner.Destination__c FROM Parcel__c]; }`
		if !compileGateReferencesSchemaObject("Probe.cls", source, units, "User") {
			t.Fatal("generated Owner target disappeared on repeated lookup")
		}
	}
	after, err := json.Marshal(index.Objects)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("relationship resolution mutated the published schema index")
	}
}

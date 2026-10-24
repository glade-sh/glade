package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaReferenceGateRelationshipTypeofElseRoutes(t *testing.T) {
	for _, object := range []string{"Task", "Event"} {
		t.Run(object, func(t *testing.T) {
			index := typesys.Index{Objects: schemaRelationshipGateMetadata("Lead").Objects}
			source := `public class Probe { Object items = [SELECT TYPEOF Who WHEN Contact THEN Name ELSE Name END FROM ` + object + `]; }`
			if !compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), "Lead") {
				t.Fatal("ELSE must retain unselected polymorphic targets")
			}
		})
	}
}

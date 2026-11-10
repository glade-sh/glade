package typesys

import (
	"github.com/glade-sh/glade/internal/schema"
	"testing"
)

func TestDependencyFieldSetSnapshotDoesNotCloseHostInventory(t *testing.T) {
	for _, namespace := range []string{"", "vendor"} {
		t.Run(namespace, func(t *testing.T) {
			input := schema.Schema{Objects: []schema.Object{{Name: "Account", FieldSetsComplete: true, FieldSetNames: []string{"Summary"}}, {Name: "Thing__c", FieldSetsComplete: true, FieldSetNames: []string{"Main"}}}}
			output := namespaceDependencySchema(input, namespace)
			for i, object := range output.Objects {
				if object.FieldSetsComplete {
					t.Errorf("dependency object %s closes host field sets", object.Name)
				}
				if !input.Objects[i].FieldSetsComplete {
					t.Error("source dependency mutated")
				}
				if len(object.FieldSetNames) != 1 {
					t.Error("known names discarded")
				}
			}
		})
	}
}

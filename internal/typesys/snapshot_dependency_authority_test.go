package typesys

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func TestDependencySnapshotStandardAuthority(t *testing.T) {
	for _, namespace := range []string{"", "fixture"} {
		t.Run(namespace, func(t *testing.T) {
			input := schema.Schema{Objects: []schema.Object{{Name: "Account", FieldsComplete: true, CompleteSnapshot: true}, {Name: "Contact", FieldsComplete: true, CompleteSnapshot: true}}}
			output := namespaceDependencySchema(input, namespace)
			for i, object := range output.Objects {
				if object.FieldsComplete || object.CompleteSnapshot {
					t.Errorf("dependency standard object %s claims host authority", object.Name)
				}
				if !input.Objects[i].FieldsComplete || !input.Objects[i].CompleteSnapshot {
					t.Errorf("source dependency object was mutated")
				}
			}
		})
	}
}

func TestDependencyArtifactSnapshotStandardAuthority(t *testing.T) {
	input := []schema.Object{
		{Name: "Account", FieldsComplete: true, CompleteSnapshot: true},
		{Name: "Contact", FieldsComplete: true, CompleteSnapshot: true},
		{Name: "Fixture__c", FieldsComplete: true, CompleteSnapshot: true},
	}
	output := clearDependencyStandardSnapshotAuthority(input)
	for i := 0; i < 2; i++ {
		if output[i].FieldsComplete || output[i].CompleteSnapshot {
			t.Errorf("artifact standard object %s claims host authority", output[i].Name)
		}
	}
	if !output[2].FieldsComplete || !output[2].CompleteSnapshot {
		t.Error("custom object authority was changed")
	}
	if !input[0].FieldsComplete || !input[1].CompleteSnapshot {
		t.Error("artifact source objects were mutated")
	}
}

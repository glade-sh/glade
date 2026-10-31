package storage

import (
	"strings"
	"testing"
)

// A no-op decision must observe edits to the current definition rather than
// caching a verdict across calls. These cases also retain case-folded lookup.
func TestOptionalFieldNoopGateObservesDefinitionEdits(t *testing.T) {
	for _, objectName := range []string{"Account", "Contact"} {
		t.Run(objectName, func(t *testing.T) {
			org := NewOrgState()
			org.Features = []string{}
			EnsureStandardObject(&org, objectName)
			definition := org.Objects[objectName].Definition.Clone()
			signature := definition.Metadata[standardFieldsOverlayMarker]
			if standardOptionalFieldsNeedRepair(definition, signature) {
				t.Fatal("prepared definition unexpectedly needs repair")
			}
			policy := standardFieldPolicyFor(objectName)
			for _, gate := range policy.gates {
				if standardFieldPolicyGateEnabledForDefinition(gate, definition, signature) || standardFieldPolicyPreservesConfiguredField(definition, gate.Field.Name) {
					continue
				}
				if _, exists := ResolveFieldName(definition, "", gate.Field.Name); exists {
					continue
				}
				key := strings.ToLower(gate.Field.Name)
				definition.Fields[key] = Field{APIName: key, Type: FieldString}
				if !standardOptionalFieldsNeedRepair(definition, signature) {
					t.Fatalf("adding disabled field %q did not require repair", key)
				}
				delete(definition.Fields, key)
				if standardOptionalFieldsNeedRepair(definition, signature) {
					t.Fatalf("removing %q did not restore the no-op", key)
				}
				return
			}
			t.Fatal("fixture has no disabled optional field")
		})
	}
}

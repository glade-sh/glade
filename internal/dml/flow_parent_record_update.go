package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// flowParentRelationshipObject resolves $Record.<Relationship> to the single
// object its lookup field references.
func (e *Engine) flowParentRelationshipObject(reference string, sourceDefinition storage.ObjectDefinition) (string, bool) {
	reference = strings.TrimSpace(reference)
	explicitParent := false
	for _, prefix := range []string{"$Record.", "Record."} {
		if strings.HasPrefix(reference, prefix) {
			reference = strings.TrimPrefix(reference, prefix)
			explicitParent = true
			break
		}
	}
	if !explicitParent || reference == "" || strings.Contains(reference, ".") || e.Org == nil {
		return "", false
	}
	field, ok := relationshipLookupField(sourceDefinition, e.Org.Namespace, reference)
	if !ok || len(field.ReferenceTo) != 1 {
		return "", false
	}
	return field.ReferenceTo[0], true
}

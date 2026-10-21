package automation

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// A related-record update can name a triggering-record relationship without
// repeating its target object. Resolve standard relationships from describe
// metadata before lowering the update; relationship names need not be object
// names, and polymorphic relationships do not identify a single target.
func flowResolveStandardRelationshipUpdates(raw *flowXML) {
	if len(raw.RecordUpdates) == 0 || !strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordAfterSave") {
		return
	}
	definition, ok := storage.StandardObjectDefinition(strings.TrimSpace(raw.Start.Object))
	if !ok {
		return
	}
	for i := range raw.RecordUpdates {
		update := &raw.RecordUpdates[i]
		if strings.TrimSpace(update.Object) != "" {
			continue
		}
		reference := strings.TrimSpace(update.InputReference)
		relationship := ""
		for _, prefix := range []string{"$Record.", "Record."} {
			if strings.HasPrefix(reference, prefix) {
				relationship = strings.TrimPrefix(reference, prefix)
				break
			}
		}
		if relationship == "" || strings.Contains(relationship, ".") {
			continue
		}
		for _, field := range definition.Fields {
			if field.Type != storage.FieldReference || len(field.ReferenceTo) != 1 || !strings.EqualFold(field.RelationshipName, relationship) {
				continue
			}
			update.Object = field.ReferenceTo[0]
			break
		}
	}
}

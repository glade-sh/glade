package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func customSetNullLookup(definition storage.ObjectDefinition, field string) bool {
	if !strings.HasSuffix(strings.ToLower(definition.APIName), "__c") || !strings.HasSuffix(strings.ToLower(field), "__c") {
		return false
	}
	metadata, ok := definition.Fields[field]
	if !ok || metadata.Type != storage.FieldReference || metadata.MasterDetail {
		return false
	}
	for _, relation := range definition.Relations {
		if strings.EqualFold(relation.Field, field) && relation.SetNullOnDelete {
			return true
		}
	}
	return false
}

func (e *Engine) rememberClearedLookup(record *storage.Record, definition storage.ObjectDefinition, field string, parent storage.ID) {
	if e.LookupHistoryScope == nil || !customSetNullLookup(definition, field) {
		return
	}
	record.RememberClearedLookup(e.LookupHistoryScope, field, parent)
}

func (e *Engine) canRestoreClearedLookup(definition storage.ObjectDefinition, record storage.Record, field string, parent storage.ID) bool {
	if e.LookupHistoryScope == nil || record.ID == "" || !customSetNullLookup(definition, field) {
		return false
	}
	object, ok := e.Org.Objects[definition.APIName]
	if !ok {
		return false
	}
	_, stored, ok := storage.LookupRecordByID(object.Records, record.ID)
	if !ok || stored.System.IsDeleted {
		return false
	}
	previous, found := stored.ClearedLookupForScope(e.LookupHistoryScope, field)
	return found && previous != "" && storage.IDsEqual(previous, parent)
}

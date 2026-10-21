package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// A direct lookup null check compares its nullness with the Boolean operand.
// Keep other field kinds, relationship paths and null operators unchanged.
func flowLookupNotNullDecisionValue(item storage.WorkflowCriteriaItem, source storage.Record, definition storage.ObjectDefinition, namespace string) (storage.Value, bool) {
	if !strings.EqualFold(strings.TrimSpace(item.Operation), "isnull") || !strings.EqualFold(trimFormulaLiteral(item.Value), "false") || strings.Contains(item.Field, ".") {
		return storage.Value{}, false
	}
	name, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(item.Field))
	if !ok {
		return storage.Value{}, false
	}
	field := definition.Fields[name]
	if field.Type != storage.FieldReference || field.MasterDetail || len(field.ReferenceTo) != 1 {
		return storage.Value{}, false
	}
	value, found := sourceRecordFieldValue(source, name)
	if !found {
		return storage.NullValue(), true
	}
	return value.Clone(), true
}

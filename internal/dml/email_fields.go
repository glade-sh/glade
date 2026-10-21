package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// NormalizeEmailFields updates the DML record copy, leaving the caller's
// SObject untouched. Captured insert/update rows observe ASCII email casing
// normalized before before-trigger execution and in the stored record.
func NormalizeEmailFields(definition storage.ObjectDefinition, namespace string, record *storage.Record) {
	for name, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, name)
		if ok {
			record.Fields[name] = normalizeEmailFieldValue(definition.Fields[canonical], value)
		}
	}
}

func normalizeEmailFieldValue(field storage.Field, value storage.Value) storage.Value {
	if value.Kind != storage.ValueString || !strings.EqualFold(field.DisplayType, "EMAIL") {
		return value
	}
	// The native rows settle ASCII case only; preserve other characters.
	value.String = strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, value.String)
	return value
}

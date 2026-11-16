package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// normalizeTextWrites applies the collation capture (re_collation, API 62.0 and
// 67.0): a Text field write is trimmed of leading and trailing whitespace, and
// a value that is empty afterwards is stored as null. Only plain Text fields
// were captured (Account Name and Site, custom object Name); text areas,
// picklists, email, phone and URL fields keep their existing behavior.
func normalizeTextWrites(definition storage.ObjectDefinition, namespace string, record *storage.Record) {
	if record == nil {
		return
	}
	for name, value := range record.Fields {
		if value.Kind != storage.ValueString {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, name)
		if !ok || !isPlainTextWriteField(definition.Fields[canonical]) {
			continue
		}
		trimmed := strings.TrimFunc(value.String, func(r rune) bool { return r <= ' ' })
		if trimmed == "" {
			delete(record.Fields, name)
			if record.ExplicitNulls == nil {
				record.ExplicitNulls = map[string]bool{}
			}
			record.ExplicitNulls[name] = true
			continue
		}
		if trimmed != value.String {
			value.String = trimmed
			record.Fields[name] = value
		}
	}
}

func isPlainTextWriteField(field storage.Field) bool {
	if field.Type != storage.FieldString || field.LongTextArea || field.AutoNumber || field.Encrypted || field.Formula != "" {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
	case "", "STRING":
		return true
	}
	return false
}

package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// textWriteTrimSet is the captured trim set: space and tab. Base already trims
// spaces on STRING fields; other control characters (CR, LF, VT, FF, NUL and
// the rest of C0) were not captured and keep base behavior.
const textWriteTrimSet = " \t"

// NormalizeTextWrites trims space and tab at the edges of plain Text fields
// and converts blank values to null for DML storage.
func NormalizeTextWrites(definition storage.ObjectDefinition, namespace string, record *storage.Record, matchField string) {
	normalizeTextWrites(definition, namespace, record, matchField, false)
}

// NormalizeTriggerTextWrites also trims Text Area and Long Text Area fields
// on the working DML records. The canonical upsert match field is excluded.
func NormalizeTriggerTextWrites(definition storage.ObjectDefinition, namespace string, record *storage.Record, matchField string) {
	normalizeTextWrites(definition, namespace, record, matchField, true)
}

func normalizeTextWrites(definition storage.ObjectDefinition, namespace string, record *storage.Record, matchField string, includeTextAreas bool) {
	if record == nil {
		return
	}
	for name, value := range record.Fields {
		if value.Kind != storage.ValueString {
			continue
		}
		if !hasTrimEdge(value.String) {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, name)
		if !ok || canonical == matchField {
			continue
		}
		field := definition.Fields[canonical]
		if !isPlainTextWriteField(field) && !(includeTextAreas && isTextAreaWriteField(field)) {
			continue
		}
		trimmed := strings.Trim(value.String, textWriteTrimSet)
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

func isTextAreaWriteField(field storage.Field) bool {
	if field.Type != storage.FieldString || field.Encrypted || field.Formula != "" {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
	case "TEXTAREA", "LONGTEXTAREA":
		return true
	}
	return false
}

// hasTrimEdge reports whether text starts or ends with a trimmed character.
func hasTrimEdge(text string) bool {
	if text == "" {
		return false
	}
	return strings.IndexByte(textWriteTrimSet, text[0]) >= 0 || strings.IndexByte(textWriteTrimSet, text[len(text)-1]) >= 0
}

package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// textWriteTrimSet is the captured trim set: space and tab. Base already trims
// spaces on STRING fields; other control characters (CR, LF, VT, FF, NUL and
// the rest of C0) were not captured and keep base behavior.
const textWriteTrimSet = " \t"

// NormalizeTextWrites applies the collation capture (re_collation, API 62.0 and
// 67.0): a Text field write is trimmed of leading and trailing space and tab,
// and a value that is empty afterwards is stored as null. Only plain Text fields
// were captured (Account Name and Site, custom object Name); text areas,
// picklists, email, phone and URL fields keep their existing behavior.
// The canonical match field can be excluded while preparing an upsert trigger.
func NormalizeTextWrites(definition storage.ObjectDefinition, namespace string, record *storage.Record, matchField string) {
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
		if !ok || canonical == matchField || !isPlainTextWriteField(definition.Fields[canonical]) {
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

// hasTrimEdge reports whether text starts or ends with a trimmed character.
func hasTrimEdge(text string) bool {
	if text == "" {
		return false
	}
	return strings.IndexByte(textWriteTrimSet, text[0]) >= 0 || strings.IndexByte(textWriteTrimSet, text[len(text)-1]) >= 0
}

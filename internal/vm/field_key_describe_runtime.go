package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// FieldKey 021 captures the Id descriptor of an ordinary source-backed custom
// object with a Text record name. Do not infer event, setting or metadata Id
// facets from the same field spelling.
func fieldKeyIdDescribeFacets(definition storage.ObjectDefinition, field storage.Field) storage.Field {
	if field.Type == storage.FieldID && strings.EqualFold(field.APIName, "Id") &&
		strings.HasSuffix(strings.ToLower(definition.APIName), "__c") &&
		definition.Metadata["kind"] == "" && definition.Metadata["publishBehavior"] == "" &&
		strings.EqualFold(definition.Metadata["nameFieldType"], "Text") {
		field.IDLookup = true
		field.Nillable = storage.BoolFlag(false)
		field.Createable = storage.BoolFlag(false)
		field.Updateable = storage.BoolFlag(false)
	}
	return field
}

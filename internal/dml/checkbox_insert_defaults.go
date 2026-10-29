package dml

import "github.com/glade-sh/glade/internal/storage"

// Catalog-derived required flags can describe non-nillable checkboxes. An
// omitted checkbox with no declared default receives false before validation.
func applyOmittedRequiredCheckboxDefaults(definition storage.ObjectDefinition, record *storage.Record) {
	for name, field := range definition.Fields {
		if field.Type != storage.FieldBoolean || !field.Required ||
			!storage.FieldFlagValue(field.Createable, false) || storage.FieldFlagValue(field.Nillable, true) ||
			field.DefaultValue != "" || field.Formula != "" {
			continue
		}
		if _, present := record.GetField(name); present || record.HasExplicitNull(name) {
			continue
		}
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		record.Fields[name] = storage.BooleanValue(false)
	}
}

package dml

import "github.com/glade-sh/glade/internal/storage"

func (e *Engine) validateInsertFieldWriteability(definition storage.ObjectDefinition, record storage.Record) error {
	if storage.IsGeneratedHistoryObject(definition) {
		if !e.Options.AllowTestHistoryInsert {
			return dmlErrorf("INVALID_TYPE", nil, "dml: generated history object %s is read-only outside the supported test insert", definition.APIName)
		}
		for _, name := range []string{"ParentId", "Field"} {
			value, present := record.GetField(name)
			if !present || value.Kind == storage.ValueNull || (value.Kind == storage.ValueString && value.String == "") || (value.Kind == storage.ValueID && value.ID == "") {
				return dmlErrorf("INVALID_API_INPUT", nil, "Invalid null value for field: %s", name)
			}
		}
		// The test-only seed accepts ParentId and Field despite describe's
		// read-only facets. Validate every other explicitly populated field.
		record = record.Clone()
		delete(record.Fields, "ParentId")
		delete(record.Fields, "Field")
		delete(record.ExplicitNulls, "ParentId")
		delete(record.ExplicitNulls, "Field")
	}
	return validateFieldWriteability(definition, e.Org.Namespace, record, true)
}

package vm

import "github.com/glade-sh/glade/internal/storage"

// Native bare Developer-baseline describes omit Division and RecordTypeId
// (standard_describe_profiles/P0/historical-extra-fields.tsv, and
// standard_recordtype_field_policy/org.tsv:2-4,7-9). These fixtures exercise
// behavior that needs the field, so they configure it explicitly, as a project
// with a record type or divisions would. Configured fields are never removed.
func configureStandardField(org *storage.OrgState, object, field string) {
	state := org.Objects[object]
	if state.Definition.APIName == "" {
		state.Definition.APIName = object
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = map[string]storage.Field{}
	}
	switch field {
	case "RecordTypeId":
		state.Definition.Fields[field] = storage.Field{APIName: field, Label: "Record Type ID", Type: storage.FieldReference,
			ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}
	case "Division":
		state.Definition.Fields[field] = storage.Field{APIName: field, Label: "Division", Type: storage.FieldPicklist}
	}
	org.Objects[object] = state
}

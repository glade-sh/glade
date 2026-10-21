package schema

import "strings"

// WithGeneratedHistoryObjects adds the platform-owned table for custom objects
// that explicitly enable history. A supplied describe object takes precedence.
func WithGeneratedHistoryObjects(objects []Object) []Object {
	seen := make(map[string]bool, len(objects))
	for _, object := range objects {
		seen[strings.ToLower(object.Name)] = true
	}
	out := append([]Object(nil), objects...)
	for _, object := range objects {
		if !object.EnableHistory || object.CustomSettingsType != "" || !strings.HasSuffix(strings.ToLower(object.Name), "__c") {
			continue
		}
		name := object.Name[:len(object.Name)-3] + "__History"
		if seen[strings.ToLower(name)] {
			continue
		}
		label := object.Label
		if label == "" {
			label = object.Name
		}
		flag := func(value bool) *bool { return &value }
		field := func(name, kind string, nullable, defaulted bool) Field {
			return Field{Name: name, Type: kind, Nillable: flag(nullable), DefaultedOnCreate: flag(defaulted), Createable: flag(false), Updateable: flag(false)}
		}
		parent := field("ParentId", "Lookup", false, false)
		parent.ReferenceTo = []string{object.Name}
		parent.RelationshipName = "Parent"
		createdBy := field("CreatedById", "Lookup", false, true)
		createdBy.ReferenceTo = []string{"User"}
		createdBy.RelationshipName = "CreatedBy"
		out = append(out, Object{
			Name: name, Label: "History: " + label, PluralLabel: "History: " + label,
			GeneratedHistoryParent: object.Name,
			Fields: []Field{
				field("Id", "Id", false, true), field("IsDeleted", "Checkbox", false, true),
				parent, createdBy, field("CreatedDate", "DateTime", false, true),
				field("Field", "Picklist", false, false), field("DataType", "Picklist", true, false),
				field("OldValue", "AnyType", true, false), field("NewValue", "AnyType", true, false),
			},
		})
		seen[strings.ToLower(name)] = true
	}
	return out
}

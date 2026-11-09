package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// WithoutAnonymousHiddenFields returns index without the object fields that
// anonymous Apex cannot see (vm.AnonymousPolicyHiddenFields), so static SOQL
// and SObject references to them fail to compile as unknown fields. Native
// admin_optional_field_api_matrix anonStatic rows: compileError with no field
// grant, compiled with one. The input index is not modified.
func WithoutAnonymousHiddenFields(index typesys.Index, hidden map[string][]string) typesys.Index {
	if len(hidden) == 0 {
		return index
	}
	prefix := ""
	if index.Project.Namespace != "" {
		prefix = strings.ToLower(index.Project.Namespace) + "__"
	}
	local := func(name string) string {
		name = strings.ToLower(name)
		if prefix != "" {
			name = strings.TrimPrefix(name, prefix)
		}
		return name
	}
	byObject := make(map[string]map[string]bool, len(hidden))
	for object, fields := range hidden {
		set := make(map[string]bool, len(fields))
		for _, field := range fields {
			set[local(field)] = true
		}
		byObject[local(object)] = set
	}
	objects := make([]schema.Object, len(index.Objects))
	copy(objects, index.Objects)
	changed := false
	for i, object := range objects {
		set := byObject[local(object.Name)]
		if len(set) == 0 {
			continue
		}
		fields := make([]schema.Field, 0, len(object.Fields))
		for _, field := range object.Fields {
			if !set[local(field.Name)] {
				fields = append(fields, field)
			}
		}
		if len(fields) != len(object.Fields) {
			objects[i].Fields = fields
			changed = true
		}
	}
	if !changed {
		return index
	}
	index.Objects = objects
	return index
}

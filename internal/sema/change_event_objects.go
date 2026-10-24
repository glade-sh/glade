package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func semaWithCustomChangeEventObjects(objects []schema.Object) []schema.Object {
	seen := make(map[string]bool, len(objects))
	for _, object := range objects {
		seen[normalizeName(object.Name)] = true
	}
	out := append([]schema.Object(nil), objects...)
	for _, object := range objects {
		if !strings.HasSuffix(object.Name, "__c") || object.CustomSettingsType != "" {
			continue
		}
		name := strings.TrimSuffix(object.Name, "__c") + "__ChangeEvent"
		if seen[normalizeName(name)] {
			continue
		}
		var fields []schema.Field
		for _, field := range object.Fields {
			if storage.ChangeEventExcludesBaseField(field.Name) {
				continue
			}
			// Event objects never own a parent's child relationship.
			field.ChildRelationshipName = ""
			field.ChildRelationshipNameInferred = false
			fields = append(fields, field)
		}
		for _, field := range semaCommonSObjectSchemaFields(object) {
			if !storage.ChangeEventExcludesBaseField(field.Name) {
				field.ChildRelationshipName = ""
				fields = semaMergeSchemaField(fields, field)
			}
		}
		fields = semaMergeSchemaField(fields, schema.Field{Name: "ChangeEventHeader", Type: "EventBus.ChangeEventHeader"})
		fields = semaMergeSchemaField(fields, schema.Field{Name: "ReplayId", Type: "Text"})
		out = append(out, schema.Object{Name: name, Fields: fields, Partial: object.Partial})
		seen[normalizeName(name)] = true
	}
	return out
}

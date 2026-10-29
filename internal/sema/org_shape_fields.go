package sema

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Reuse the runtime org-shape field model against an isolated schema view.
// Features come from the immutable index, not the live project filesystem.
func enrichIndexWithOrgShapeFields(index typesys.Index) typesys.Index {
	if len(index.OrgShapeFeatures) == 0 || len(index.OrgShapeFeatures) == 1 && index.OrgShapeFeatures[0] == project.UnspecifiedStandardFieldShape {
		return index
	}
	registry := sobject.BuildDescribeRegistry(schema.Schema{Objects: index.Objects})
	org := storage.NewOrgState()
	org.Namespace = index.Project.Namespace
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
	}
	featureObjects := storage.StandardFeatureObjectNames(index.OrgShapeFeatures)
	for _, name := range featureObjects {
		storage.EnsureStandardObject(&org, name)
	}
	storage.ApplyProjectOrgShape(&org, index.OrgShapeFeatures)
	objects := append([]schema.Object(nil), index.Objects...)
	knownObjects := make(map[string]bool, len(objects))
	for _, object := range objects {
		knownObjects[normalizeName(object.Name)] = true
	}
	for _, name := range featureObjects {
		if !knownObjects[normalizeName(name)] {
			objects = append(objects, schema.Object{Name: name, Partial: true})
		}
	}
	for i, object := range objects {
		state, ok := org.Objects[object.Name]
		if !ok {
			continue
		}
		// Preserve original declarations and metadata; add the shared synthetic
		// fields only to the prepared semantic view.
		fields := append([]schema.Field(nil), object.Fields...)
		known := make(map[string]bool, len(fields))
		for _, field := range fields {
			known[normalizeName(field.Name)] = true
		}
		for _, field := range semaSchemaFieldsFromStandardDefinition(state.Definition) {
			if !known[normalizeName(field.Name)] {
				fields = append(fields, field)
				known[normalizeName(field.Name)] = true
			}
		}
		objects[i].Fields = fields
	}
	index.Objects = objects
	return index
}

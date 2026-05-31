package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) validateDeleteReferences(objectName string, id storage.ID, ctx *deleteContext) error {
	relations := e.restrictedDeleteRelations(objectName, ctx)
	for _, relation := range relations {
		childrenByParent := e.referenceIndexForRelation(relation.childObject, relation.field, ctx)
		for _, childID := range childrenByParent[id] {
			childObject := e.Org.Objects[relation.childObject]
			childRecord, ok := childObject.Records[childID]
			if !ok || childRecord.System.IsDeleted {
				continue
			}
			value, ok := childRecord.Fields[relation.field]
			if ok && idFromStorageValue(value) == id {
				return dmlErrorf("DELETE_FAILED", []string{relation.field}, "dml: cannot delete %s %s because %s records reference it", objectName, id, relation.childObject)
			}
		}
	}
	return nil
}

func (e *Engine) cascadeDeleteChildren(objectName string, id storage.ID, seen map[string]bool, ctx *deleteContext) error {
	relations := e.cascadeDeleteRelations(objectName, ctx)
	for _, relation := range relations {
		childrenByParent := e.referenceIndexForRelation(relation.childObject, relation.field, ctx)
		for _, childID := range childrenByParent[id] {
			childObject := e.Org.Objects[relation.childObject]
			childRecord, ok := childObject.Records[childID]
			if !ok || childRecord.System.IsDeleted {
				continue
			}
			value, ok := childRecord.Fields[relation.field]
			if ok && idFromStorageValue(value) == id {
				if err := e.deleteRecord(relation.childObject, childID, seen, ctx); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (e *Engine) buildDeleteContext() *deleteContext {
	if e == nil || e.Org == nil {
		return nil
	}
	ctx := &deleteContext{
		restrictedByParent: make(map[string][]deleteRelation),
		cascadeByParent:    make(map[string][]deleteRelation),
		referenceIndex:     make(map[string]map[storage.ID][]storage.ID),
	}
	for childObjectName, childObject := range e.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relation.RestrictedDelete && !relation.CascadeDelete {
				continue
			}
			index := make(map[storage.ID][]storage.ID)
			for childID, child := range childObject.Records {
				if child.System.IsDeleted {
					continue
				}
				value, ok := child.Fields[relation.Field]
				if !ok {
					continue
				}
				parentID := idFromStorageValue(value)
				if parentID == "" {
					continue
				}
				index[parentID] = append(index[parentID], childID)
			}
			ctx.referenceIndex[deleteRelationKey(childObjectName, relation.Field)] = index
			for _, parentObject := range relation.ParentObjects {
				rel := deleteRelation{childObject: childObjectName, field: relation.Field}
				if relation.RestrictedDelete {
					ctx.restrictedByParent[parentObject] = append(ctx.restrictedByParent[parentObject], rel)
				}
				if relation.CascadeDelete {
					ctx.cascadeByParent[parentObject] = append(ctx.cascadeByParent[parentObject], rel)
				}
			}
		}
	}
	return ctx
}

func deleteRelationKey(childObject, field string) string {
	return childObject + "|" + field
}

func (e *Engine) restrictedDeleteRelations(objectName string, ctx *deleteContext) []deleteRelation {
	if ctx != nil {
		if relations, ok := ctx.restrictedByParent[objectName]; ok {
			return relations
		}
		return nil
	}
	out := make([]deleteRelation, 0)
	for childObjectName, childObject := range e.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if relation.RestrictedDelete && containsString(relation.ParentObjects, objectName) {
				out = append(out, deleteRelation{childObject: childObjectName, field: relation.Field})
			}
		}
	}
	return out
}

func (e *Engine) cascadeDeleteRelations(objectName string, ctx *deleteContext) []deleteRelation {
	if ctx != nil {
		if relations, ok := ctx.cascadeByParent[objectName]; ok {
			return relations
		}
		return nil
	}
	out := make([]deleteRelation, 0)
	for childObjectName, childObject := range e.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if relation.CascadeDelete && containsString(relation.ParentObjects, objectName) {
				out = append(out, deleteRelation{childObject: childObjectName, field: relation.Field})
			}
		}
	}
	return out
}

func (e *Engine) referenceIndexForRelation(childObject, field string, ctx *deleteContext) map[storage.ID][]storage.ID {
	if ctx != nil {
		if index, ok := ctx.referenceIndex[deleteRelationKey(childObject, field)]; ok {
			return index
		}
		return nil
	}
	childState := e.Org.Objects[childObject]
	index := make(map[storage.ID][]storage.ID)
	for childID, child := range childState.Records {
		if child.System.IsDeleted {
			continue
		}
		value, ok := child.Fields[field]
		if !ok {
			continue
		}
		parentID := idFromStorageValue(value)
		if parentID == "" {
			continue
		}
		index[parentID] = append(index[parentID], childID)
	}
	return index
}

func upsertExternalID(definition storage.ObjectDefinition, namespace string, record storage.Record, externalIDField string) (string, storage.Value, bool, error) {
	if externalIDField != "" {
		fieldName := externalIDField
		if canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName); ok {
			fieldName = canonical
		}
		field, ok := definition.Fields[fieldName]
		if !ok {
			return "", storage.Value{}, false, dmlErrorf("INVALID_FIELD", []string{externalIDField}, "dml: unknown external id field %s.%s", record.Object, externalIDField)
		}
		if !field.ExternalID && !(strings.EqualFold(fieldName, "Name") && storage.IsCustomSettingDefinition(definition)) {
			return "", storage.Value{}, false, dmlErrorf("INVALID_FIELD", []string{fieldName}, "dml: field %s.%s is not an external id", record.Object, fieldName)
		}
		value, ok := record.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			return fieldName, storage.Value{}, false, dmlErrorf("MISSING_ARGUMENT", []string{fieldName}, "dml: external id field %s.%s is missing", record.Object, fieldName)
		}
		return fieldName, value, true, nil
	}
	return "", storage.Value{}, false, nil
}

func idFromStorageValue(value storage.Value) storage.ID {
	switch value.Kind {
	case storage.ValueID:
		return value.ID
	case storage.ValueString:
		return storage.ID(value.String)
	default:
		return ""
	}
}

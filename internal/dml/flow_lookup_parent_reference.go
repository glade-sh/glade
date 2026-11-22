package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// flowLookupParentReferenceValue reads a Get Records filter value of the form
// $Record.<Relationship>.<ReferenceField> (lowered to Relationship.Field): one
// non-polymorphic parent hop to a lookup field of the parent. A null
// relationship, or a parent without the value, gives null. Captured for an
// after-save create Flow in flow_criteria_shapes/org.tsv (rv rows, 62.0 and
// 67.0, both routes); other field types and deeper paths stay unsupported.
func (e *Engine) flowLookupParentReferenceValue(sourceField string, source storage.Record, sourceDefinition storage.ObjectDefinition) (storage.Value, bool) {
	if e == nil || e.Org == nil {
		return storage.Value{}, false
	}
	relationship, name, nested := strings.Cut(strings.TrimSpace(sourceField), ".")
	if !nested || relationship == "" || name == "" || strings.Contains(name, ".") || strings.HasPrefix(relationship, "$") {
		return storage.Value{}, false
	}
	for apiName, field := range sourceDefinition.Fields {
		if field.Type != storage.FieldReference || !dmlRelationshipNameMatches(e.Org.Namespace, storage.ParentRelationshipName(field), relationship) {
			continue
		}
		if len(field.ReferenceTo) != 1 {
			return storage.Value{}, false
		}
		parentName, ok := storage.ResolveObjectName(*e.Org, field.ReferenceTo[0])
		if !ok {
			return storage.Value{}, false
		}
		parent := e.Org.Objects[parentName]
		parentField, ok := storage.ResolveFieldName(parent.Definition, e.Org.Namespace, name)
		if !ok || parent.Definition.Fields[parentField].Type != storage.FieldReference {
			return storage.Value{}, false
		}
		id, present := sourceRecordFieldValue(source, apiName)
		if !present || id.Kind == storage.ValueNull {
			return storage.NullValue(), true
		}
		record, found := parent.Records[idFromStorageValue(id)]
		if !found {
			return storage.Value{}, false
		}
		value, present := sourceRecordFieldValue(record, parentField)
		if !present {
			return storage.NullValue(), true
		}
		return value.Clone(), true
	}
	return storage.Value{}, false
}

// flowFrameSObjectVariable reports a declared single SObject variable.
func flowFrameSObjectVariable(frame *flowFrame, key string) bool {
	if frame == nil {
		return false
	}
	variable, declared := frame.variables[key]
	return declared && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject")
}

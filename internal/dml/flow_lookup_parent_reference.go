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
		parentID := idFromStorageValue(id)
		// Parent maps normally use 15-character keys. Avoid the shared
		// lookup's linear equivalent-ID scan for the common 18-character form
		// without changing exact-key precedence or the fallback semantics.
		if len(parentID) == 18 {
			if _, exact := parent.Records[parentID]; !exact {
				short := parentID[:15]
				if _, present := parent.Records[short]; present {
					parentID = short
				}
			}
		}
		_, record, found := storage.LookupRecordByID(parent.Records, parentID)
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

// flowLookupFormulaCriterion compares a Text field of a custom metadata record
// with a String formula value evaluated on the interview, case-insensitively
// and null-safe: flow_metadata_formula_lookup/org.tsv (before-save create,
// 62.0 and 67.0, both routes). Any other target or field type is unsupported.
func (e *Engine) flowLookupFormulaCriterion(item storage.WorkflowCriteriaItem, target storage.Record, targetDefinition storage.ObjectDefinition, frame *flowFrame) (bool, bool) {
	if frame == nil || e.Org == nil || !strings.EqualFold(strings.TrimSpace(item.Operation), "equals") ||
		!strings.HasSuffix(strings.ToLower(targetDefinition.APIName), "__mdt") {
		return false, false
	}
	name, ok := storage.ResolveFieldName(targetDefinition, e.Org.Namespace, strings.TrimSpace(item.Field))
	if !ok || targetDefinition.Fields[name].Type != storage.FieldString {
		return false, false
	}
	value, ok := e.flowFrameFormulaValue(item.Formula, frame, false)
	if !ok {
		return false, false
	}
	stored, present := sourceRecordFieldValue(target, name)
	if value.Kind == storage.ValueNull || !present || stored.Kind == storage.ValueNull {
		return false, true
	}
	if value.Kind != storage.ValueString || stored.Kind != storage.ValueString {
		return false, false
	}
	return strings.EqualFold(value.String, stored.String), true
}

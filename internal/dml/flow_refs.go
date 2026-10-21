package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Relationship text values include Text and Picklist fields reached through
// one or two parent relationships. Validate the entire schema path
// before reading records; a missing valid parent is null, an unknown path is
// unsupported. Formula fields and deeper paths keep their existing behavior.
func (e *Engine) flowRefsRelationshipValue(reference string, record storage.Record, definition storage.ObjectDefinition) (storage.Value, bool) {
	parts := strings.Split(strings.TrimSpace(reference), ".")
	if e == nil || e.Org == nil || len(parts) < 2 || len(parts) > 3 {
		return storage.Value{}, false
	}
	lookup, ok := relationshipLookupField(definition, e.Org.Namespace, parts[0])
	if !ok {
		return storage.Value{}, false
	}
	field, _, ok := formulaRelationshipFieldDefinition(*e.Org, lookup, parts[1:], e.Org.Namespace)
	if !ok || (field.Type != storage.FieldString && field.Type != storage.FieldPicklist) || strings.TrimSpace(field.Formula) != "" {
		return storage.Value{}, false
	}
	value, ok := formulaRelationshipFieldValue(e.Org, definition, record, reference, e.Org.Namespace, nil)
	if !ok {
		return storage.Value{}, false
	}
	if value.kind == formulaNull {
		return storage.NullValue(), true
	}
	return storage.StringValue(value.asString()), true
}

func (e *Engine) flowRefsDecisionRelationshipValue(reference string, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (storage.Value, bool) {
	if value, ok := e.flowTypedUserBooleanDecisionValue(reference, source, definition); ok {
		return value, true
	}
	if frame != nil {
		if dot := strings.Index(reference, "."); dot > 0 {
			if output, ok := frame.lookupOutputs[flowFrameKey(reference[:dot])]; ok {
				return e.flowRefsRelationshipValue(reference[dot+1:], output.record, output.definition)
			}
		}
	}
	return e.flowRefsRelationshipValue(reference, source, definition)
}

// A typed User relationship does not match a Group owner. Resolve the declared
// relationship and Boolean field before reading the stored parent record.
func (e *Engine) flowTypedUserBooleanDecisionValue(reference string, source storage.Record, definition storage.ObjectDefinition) (storage.Value, bool) {
	if e == nil || e.Org == nil {
		return storage.Value{}, false
	}
	relationship, target, typed := strings.Cut(strings.TrimSpace(reference), ":")
	typeName, fieldName, fieldPath := strings.Cut(target, ".")
	if !typed || !fieldPath || !strings.EqualFold(typeName, "User") || strings.Contains(fieldName, ".") {
		return storage.Value{}, false
	}
	lookup, ok := relationshipLookupField(definition, e.Org.Namespace, relationship)
	if !ok || lookup.Type != storage.FieldReference {
		return storage.Value{}, false
	}
	allowed := map[string]bool{}
	for _, target := range lookup.ReferenceTo {
		allowed[strings.ToLower(target)] = true
	}
	if len(allowed) != 2 || !allowed["user"] || !allowed["group"] {
		return storage.Value{}, false
	}
	userName, ok := storage.ResolveObjectName(*e.Org, "User")
	if !ok {
		return storage.Value{}, false
	}
	userDefinition := e.Org.Objects[userName].Definition
	fieldName, ok = storage.ResolveFieldName(userDefinition, e.Org.Namespace, fieldName)
	if !ok || userDefinition.Fields[fieldName].Type != storage.FieldBoolean || userDefinition.Fields[fieldName].Formula != "" {
		return storage.Value{}, false
	}
	owner, present := sourceRecordFieldValue(source, lookup.APIName)
	if !present || owner.Kind == storage.ValueNull {
		return storage.NullValue(), true
	}
	parent, parentDefinition, found := formulaParentRecord(*e.Org, lookup, idFromStorageValue(owner))
	if !found || !strings.EqualFold(parentDefinition.APIName, userName) {
		return storage.NullValue(), true
	}
	value, present := sourceRecordFieldValue(parent, fieldName)
	if !present {
		return storage.NullValue(), true
	}
	if value.Kind != storage.ValueBoolean && value.Kind != storage.ValueNull {
		return storage.Value{}, false
	}
	return value.Clone(), true
}

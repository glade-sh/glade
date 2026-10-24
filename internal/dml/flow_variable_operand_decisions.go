package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// flowDeclaredScalar reads a declared non-collection scalar variable. An
// unassigned variable without a default reads as null.
func flowDeclaredScalar(name string, frame *flowFrame) (storage.Value, bool) {
	if frame == nil {
		return storage.Value{}, false
	}
	key := flowFrameKey(name)
	variable, declared := frame.variables[key]
	if !declared || variable.IsCollection || strings.EqualFold(variable.DataType, "SObject") {
		return storage.Value{}, false
	}
	if value, ok := frame.scalars[key]; ok {
		return value.Clone(), true
	}
	return storage.NullValue(), true
}

// A decision comparing a scalar variable with a resource under EqualTo or
// NotEqualTo. Two null operands are equal (native rows at API 62.0 and 67.0).
func (e *Engine) flowVariableOperandCriterion(item storage.WorkflowCriteriaItem, frame *flowFrame) (bool, bool, bool) {
	if value, ok := e.flowSObjectVariableFieldValue(item, frame); ok {
		matches, supported := evaluateFlowValueCriteria(value, item)
		return matches, supported, true
	}
	if strings.EqualFold(strings.TrimSpace(item.Operation), "isnull") && frame != nil {
		if left, ok := flowDeclaredScalar(item.Field, frame); ok {
			matches, supported := evaluateFlowValueCriteria(left, item)
			return matches, supported, true
		}
	}
	reference := strings.TrimSpace(item.SourceField)
	if value := strings.TrimSpace(item.Value); reference == "" && strings.HasPrefix(value, "{!") && strings.HasSuffix(value, "}") {
		reference = strings.TrimSpace(value[2 : len(value)-1])
	}
	if frame == nil || reference == "" {
		return false, false, false
	}
	operation := strings.ToLower(strings.TrimSpace(item.Operation))
	if operation != "equals" && operation != "notequal" && operation != "notequals" {
		return false, false, false
	}
	left, ok := flowDeclaredScalar(item.Field, frame)
	if !ok {
		return false, false, false
	}
	if reference == "{!$EmptyString}" || reference == "$EmptyString" {
		// An explicit empty String equals only an empty String value.
		equal := left.Kind == storage.ValueString && left.String == ""
		if operation == "equals" {
			return equal, true, true
		}
		return !equal, true, true
	}
	right, ok := flowDeclaredScalar(reference, frame)
	if !ok {
		if right, ok = e.flowFrameReferenceValue(reference, frame); !ok {
			return false, false, true
		}
	}
	equal := false
	switch {
	case left.Kind == storage.ValueNull || right.Kind == storage.ValueNull:
		equal = left.Kind == right.Kind
	default:
		item.Value = workflowValueString(right)
		matches, supported := evaluateFlowValueCriteria(left, item)
		return matches, supported, true
	}
	if operation == "equals" {
		return equal, true, true
	}
	return !equal, true, true
}

// flowSObjectVariableFieldValue reads one field of a single-record SObject
// variable under EqualTo, NotEqualTo and IsNull. A variable with no stored
// record, such as an unmatched Get Records, reads null.
func (e *Engine) flowSObjectVariableFieldValue(item storage.WorkflowCriteriaItem, frame *flowFrame) (storage.Value, bool) {
	if frame == nil || e == nil || e.Org == nil || strings.TrimSpace(item.SourceField) != "" || strings.HasPrefix(strings.TrimSpace(item.Value), "{!") {
		return storage.Value{}, false
	}
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "equals", "notequal", "notequals", "isnull":
	default:
		return storage.Value{}, false
	}
	name, field, found := strings.Cut(strings.TrimSpace(item.Field), ".")
	if !found || field == "" || strings.Contains(field, ".") || strings.HasPrefix(name, "$") {
		return storage.Value{}, false
	}
	key := flowFrameKey(name)
	variable, declared := frame.variables[key]
	if !declared || variable.IsCollection || !strings.EqualFold(variable.DataType, "SObject") {
		return storage.Value{}, false
	}
	record := frame.records[key]
	objectType := record.Object
	if objectType == "" {
		objectType = variable.ObjectType
	}
	objectName, ok := storage.ResolveObjectName(*e.Org, objectType)
	if !ok {
		return storage.Value{}, false
	}
	definition := e.Org.Objects[objectName].Definition
	fieldName, ok := storage.ResolveFieldName(definition, e.Org.Namespace, field)
	if !ok {
		return storage.Value{}, false
	}
	value, present := sourceRecordFieldValue(record, fieldName)
	if !present {
		return storage.NullValue(), true
	}
	return value.Clone(), true
}

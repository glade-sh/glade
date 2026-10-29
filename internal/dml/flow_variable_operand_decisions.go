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
	if matches, supported, handled := flowNumericVariableOperandCriterion(item, frame); handled {
		return matches, supported, true
	}
	if value, ok := e.flowSObjectVariableFieldValue(item, frame); ok {
		if reference := flowCriteriaResourceReference(item); reference != "" {
			right, found := flowDeclaredScalar(reference, frame)
			// Only String EqualTo lookup/resource comparisons have native
			// coverage (flow_decision_criteria_native/org.tsv:16,50). Leave
			// every other resource shape to the existing decision dispatcher.
			if !strings.EqualFold(strings.TrimSpace(item.Operation), "equals") || !found || value.Kind != storage.ValueString || right.Kind != storage.ValueString {
				return false, false, false
			}
			return value.String == right.String, true, true
		}
		matches, supported := evaluateFlowValueCriteria(value, item)
		return matches, supported, true
	}
	if strings.EqualFold(strings.TrimSpace(item.Operation), "isnull") && frame != nil {
		if left, ok := flowDeclaredScalar(item.Field, frame); ok {
			matches, supported := evaluateFlowValueCriteria(left, item)
			return matches, supported, true
		}
	}
	reference := flowCriteriaResourceReference(item)
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
	case left.Kind == storage.ValueString && right.Kind == storage.ValueString:
		equal = strings.EqualFold(strings.TrimSpace(left.String), strings.TrimSpace(right.String))
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

// flowSObjectVariableFieldValue resolves a field on either a declared SObject
// variable or a Get Records result. An unmatched Get Records result has null
// field values but keeps the lookup's schema for resolving the field name.
func (e *Engine) flowSObjectVariableFieldValue(item storage.WorkflowCriteriaItem, frame *flowFrame) (storage.Value, bool) {
	if frame == nil || e == nil || e.Org == nil {
		return storage.Value{}, false
	}
	name, field, found := strings.Cut(strings.TrimSpace(item.Field), ".")
	if !found || field == "" || strings.Contains(field, ".") || strings.HasPrefix(name, "$") {
		return storage.Value{}, false
	}
	key := flowFrameKey(name)
	var record storage.Record
	var definition storage.ObjectDefinition
	if output, present := frame.lookupOutputs[key]; present {
		// Only captured lookup-field operators use this reader. Other
		// operators must retain their existing dispatcher and provenance.
		switch strings.ToLower(strings.TrimSpace(item.Operation)) {
		case "equals", "notequal", "notequals", "isnull", "isblank", "contains", "startswith", "endswith":
		default:
			return storage.Value{}, false
		}
		if e.InvocableFlow && flowCriteriaResourceReference(item) != "" {
			// Invocable decisions already resolve both operands, including
			// non-String values, through invocableFlowRecordCriterion.
			return storage.Value{}, false
		}
		record = output.record
		definition = output.definition
	} else {
		// Declared records retain the existing invocable resource and operator
		// paths; the broader criteria reader is for captured lookup outputs.
		if flowCriteriaResourceReference(item) != "" {
			return storage.Value{}, false
		}
		switch strings.ToLower(strings.TrimSpace(item.Operation)) {
		case "equals", "notequal", "notequals", "isnull":
		default:
			return storage.Value{}, false
		}
		variable, declared := frame.variables[key]
		if !declared || variable.IsCollection || !strings.EqualFold(variable.DataType, "SObject") {
			return storage.Value{}, false
		}
		record = frame.records[key]
		objectName := strings.TrimSpace(record.Object)
		if objectName == "" {
			objectName = variable.ObjectType
		}
		resolved, ok := storage.ResolveObjectName(*e.Org, objectName)
		if !ok {
			return storage.Value{}, false
		}
		definition = e.Org.Objects[resolved].Definition
	}
	fieldName, ok := storage.ResolveFieldName(definition, e.Org.Namespace, field)
	if !ok {
		return storage.Value{}, false
	}
	if record.Object == "" && record.ID == "" && len(record.Fields) == 0 {
		return storage.NullValue(), true
	}
	value, present := sourceRecordFieldValue(record, fieldName)
	if !present {
		return storage.NullValue(), true
	}
	return value.Clone(), true
}

func flowCriteriaResourceReference(item storage.WorkflowCriteriaItem) string {
	if reference := strings.TrimSpace(item.SourceField); reference != "" {
		return reference
	}
	value := strings.TrimSpace(item.Value)
	if strings.HasPrefix(value, "{!") && strings.HasSuffix(value, "}") {
		return strings.TrimSpace(value[2 : len(value)-1])
	}
	return ""
}

package dml

import (
	"math/big"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The owned native matrix covers a triggering record's direct or one-parent
// Text field, or a direct Integer field, assigned to a scalar String in a
// before-save CreateAndUpdate Flow. Validate the metadata before reading a
// field; calculated fields and other source types retain their boundary.
func (e *Engine) flowRecordTextAssignmentValue(assignment storage.FlowAssignment, frame *flowFrame) (value storage.Value, supported, handled bool) {
	if e == nil || e.Org == nil || frame == nil || !frame.beforeSaveDecisions ||
		!strings.EqualFold(strings.TrimSpace(assignment.Operator), "Assign") || strings.TrimSpace(assignment.Formula) != "" {
		return storage.Value{}, false, false
	}
	variable, declared := frame.variables[flowFrameKey(assignment.Target)]
	if !declared || variable.IsCollection || !strings.EqualFold(strings.TrimSpace(variable.DataType), "String") {
		return storage.Value{}, false, false
	}
	parts := strings.Split(strings.TrimSpace(assignment.SourceField), ".")
	if (len(parts) != 2 && len(parts) != 3) || !strings.EqualFold(parts[0], "$Record") {
		return storage.Value{}, false, false
	}
	source, present := frame.records[flowFrameKey("$Record")]
	if !present {
		return storage.Value{}, false, true
	}
	objectName, modeled := storage.ResolveObjectName(*e.Org, source.Object)
	if !modeled {
		return storage.Value{}, false, true
	}
	definition := e.Org.Objects[objectName].Definition
	if len(parts) == 3 {
		value, supported = e.flowRefsRelationshipValue(strings.Join(parts[1:], "."), source, definition)
		return value, supported, true
	}
	fieldName, modeled := storage.ResolveFieldName(definition, e.Org.Namespace, strings.TrimSpace(parts[1]))
	if !modeled {
		return storage.Value{}, false, true
	}
	field := definition.Fields[fieldName]
	integerField := field.Type == storage.FieldInteger
	// A project schema can expose a standard Integer field as Number. Read
	// its canonical type without admitting arbitrary Decimal fields.
	if field.Type == storage.FieldDecimal {
		if standard, ok := storage.StandardObjectDefinition(objectName); ok {
			if name, ok := storage.ResolveFieldName(standard, "", fieldName); ok {
				integerField = standard.Fields[name].Type == storage.FieldInteger
			}
		}
	}
	if (!integerField && field.Type != storage.FieldString) || strings.TrimSpace(field.Formula) != "" {
		return storage.Value{}, false, true
	}
	value, present = sourceRecordFieldValue(source, fieldName)
	if !present {
		return storage.NullValue(), true, true
	}
	if integerField && value.Kind != storage.ValueNull {
		switch value.Kind {
		case storage.ValueInteger:
			return storage.StringValue(workflowValueString(value)), true, true
		case storage.ValueDecimal:
			if number, ok := new(big.Rat).SetString(value.Decimal); ok && number.IsInt() {
				return storage.StringValue(number.Num().String()), true, true
			}
		}
		return storage.Value{}, false, true
	}
	return value.Clone(), true, true
}

// String scalar assignments and collection Add can read a parent Text field
// without treating the parent as a separately named interview record.
func (e *Engine) flowStringAssignmentValue(assignment storage.FlowAssignment, frame *flowFrame) (storage.Value, bool) {
	if value, ok := e.flowFrameValue(assignment, frame); ok {
		return value, true
	}
	if e == nil || e.Org == nil || frame == nil || strings.TrimSpace(assignment.Formula) != "" {
		return storage.Value{}, false
	}
	variable, declared := frame.variables[flowFrameKey(assignment.Target)]
	if !declared || (variable.IsCollection && !strings.EqualFold(assignment.Operator, "Add")) || !strings.EqualFold(variable.DataType, "String") ||
		(!strings.EqualFold(assignment.Operator, "Assign") && !strings.EqualFold(assignment.Operator, "Add")) {
		return storage.Value{}, false
	}
	parts := strings.Split(strings.TrimSpace(assignment.SourceField), ".")
	if len(parts) != 3 || !strings.EqualFold(parts[0], "$Record") {
		return storage.Value{}, false
	}
	source, present := frame.records[flowFrameKey("$Record")]
	if !present {
		return storage.Value{}, false
	}
	objectName, modeled := storage.ResolveObjectName(*e.Org, source.Object)
	if !modeled {
		return storage.Value{}, false
	}
	return e.flowRefsRelationshipValue(strings.Join(parts[1:], "."), source, e.Org.Objects[objectName].Definition)
}

// The captured assignment's scalar String output is written to a Text field
// without converting its value through formula or literal syntax. A real
// source field keeps precedence over an identically named interview scalar.
func (e *Engine) flowTextScalarFieldUpdateValue(update storage.WorkflowFieldUpdate, field storage.Field, definition storage.ObjectDefinition, frame *flowFrame) (storage.Value, bool) {
	if e == nil || e.Org == nil || frame == nil || !frame.beforeSaveDecisions ||
		field.Type != storage.FieldString || strings.TrimSpace(update.Formula) != "" {
		return storage.Value{}, false
	}
	source := strings.TrimSpace(update.SourceField)
	if source == "" || strings.Contains(source, ".") {
		return storage.Value{}, false
	}
	if _, modeled := storage.ResolveFieldName(definition, e.Org.Namespace, source); modeled {
		return storage.Value{}, false
	}
	key := flowFrameKey(source)
	variable, declared := frame.variables[key]
	if !declared || variable.IsCollection || !strings.EqualFold(strings.TrimSpace(variable.DataType), "String") || !frame.assigned[key] {
		return storage.Value{}, false
	}
	value, assigned := frame.scalars[key]
	if !assigned || (value.Kind != storage.ValueString && value.Kind != storage.ValueNull) {
		return storage.Value{}, false
	}
	return value.Clone(), true
}

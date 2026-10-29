package dml

import (
	"math/big"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The owned native matrix covers a triggering record's direct or one-parent
// Text field, or a direct Integer, whole Number or Picklist, assigned to a String in a
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
	wholeNumberField := field.Type == storage.FieldDecimal && field.Scale == 0 && !integerField &&
		!strings.EqualFold(field.DisplayType, "CURRENCY") && !strings.EqualFold(field.DisplayType, "PERCENT")
	if (!integerField && !wholeNumberField && field.Type != storage.FieldString && field.Type != storage.FieldPicklist) || strings.TrimSpace(field.Formula) != "" {
		return storage.Value{}, false, true
	}
	value, present = sourceRecordFieldValue(source, fieldName)
	if !present {
		return storage.NullValue(), true, true
	}
	if wholeNumberField && value.Kind != storage.ValueNull {
		return e.flowWholeNumberText(value)
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

func (e *Engine) flowWholeNumberText(value storage.Value) (storage.Value, bool, bool) {
	// Flow uses display formatting for Number-to-String assignment. The
	// captured profile is en_US; other locales retain their existing boundary.
	user := e.Org.Objects["User"].Records[e.UserID]
	if locale := user.Fields["LocaleSidKey"]; locale.Kind == storage.ValueString && locale.String != "" && locale.String != "en_US" {
		return storage.Value{}, false, true
	}
	raw := workflowValueString(value)
	if value.Kind == storage.ValueDecimal {
		raw = value.Decimal
	}
	number, ok := new(big.Rat).SetString(raw)
	if !ok || !number.IsInt() {
		return storage.Value{}, false, true
	}
	text := number.Num().String()
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign, text = "-", text[1:]
	}
	var grouped strings.Builder
	grouped.WriteString(sign)
	for i, digit := range text {
		if i > 0 && (len(text)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	return storage.StringValue(grouped.String()), true, true
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

// Text field updates evaluate scalar formulas in their interview frame.
// Direct String scalar writes preserve the source-field precedence rule.
func (e *Engine) flowTextScalarFieldUpdateValue(update storage.WorkflowFieldUpdate, field storage.Field, definition storage.ObjectDefinition, frame *flowFrame) (storage.Value, bool) {
	if e == nil || e.Org == nil || frame == nil || field.Type != storage.FieldString {
		return storage.Value{}, false
	}
	if strings.TrimSpace(update.Formula) != "" {
		if !flowCoreFormulaUsesScalars(update.Formula, frame) {
			return storage.Value{}, false
		}
		value, ok := flowFrameFormulaValue(update.Formula, frame)
		if e.InvocableFlow {
			value, ok = e.flowContextFormulaValue(update.Formula, frame)
		}
		return value, ok && (value.Kind == storage.ValueString || value.Kind == storage.ValueNull)
	}
	// Captured Create routes include before-save assignments and after-save
	// writes from a subflow or scalar decision chain. Update-only boundaries
	// remain separate from the before-save CreateAndUpdate matrix.
	createSave := frame.saveDecisions && !frame.priorDecisions
	if !frame.beforeSaveDecisions && !createSave {
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
	if !declared || variable.IsCollection || !strings.EqualFold(strings.TrimSpace(variable.DataType), "String") || (!frame.assigned[key] && !createSave) {
		return storage.Value{}, false
	}
	value, assigned := frame.scalars[key]
	// A missing lookup output is null; subflow outputs already reside in
	// scalars even when no assignment node ran in the parent interview.
	if !assigned && createSave {
		return storage.NullValue(), true
	}
	if !assigned || (value.Kind != storage.ValueString && value.Kind != storage.ValueNull) {
		return storage.Value{}, false
	}
	return value.Clone(), true
}

// A Number interview variable holding a whole number is written to an Integer
// field unchanged in a before-save Create or CreateAndUpdate Flow (API 62.0
// and 67.0). Fractional values and other field types keep their boundary.
func (e *Engine) flowIntegerScalarFieldUpdateValue(update storage.WorkflowFieldUpdate, field storage.Field, definition storage.ObjectDefinition, frame *flowFrame) (storage.Value, bool) {
	if e == nil || e.Org == nil || frame == nil || !frame.scalarBeforeSave ||
		field.Type != storage.FieldInteger || strings.TrimSpace(update.Formula) != "" {
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
	if !declared || variable.IsCollection || !strings.EqualFold(strings.TrimSpace(variable.DataType), "Number") || !frame.assigned[key] {
		return storage.Value{}, false
	}
	value, assigned := frame.scalars[key]
	if !assigned {
		return storage.Value{}, false
	}
	switch value.Kind {
	case storage.ValueInteger:
		return value.Clone(), true
	case storage.ValueDecimal:
		rational, ok := new(big.Rat).SetString(strings.TrimSpace(value.Decimal))
		if !ok || !rational.IsInt() || !rational.Num().IsInt64() {
			return storage.Value{}, false
		}
		return storage.IntegerValue(rational.Num().Int64()), true
	}
	return storage.Value{}, false
}

// A Number or Currency interview variable written to a numeric field by a Create save flow
// is the field's value (API 62.0 and 67.0, after-save Update Records over a
// Get Records loop). A variable no node assigned is null. Integer and decimal
// fields accept the whole or fractional number unchanged; other shapes keep
// their existing boundary.
func (e *Engine) flowNumberScalarFieldUpdateValue(update storage.WorkflowFieldUpdate, field storage.Field, definition storage.ObjectDefinition, frame *flowFrame) (storage.Value, bool) {
	if e == nil || e.Org == nil || frame == nil || !(frame.saveDecisions && !frame.priorDecisions) ||
		(field.Type != storage.FieldInteger && field.Type != storage.FieldDecimal) || strings.TrimSpace(update.Formula) != "" {
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
	dataType := strings.TrimSpace(variable.DataType)
	if !declared || variable.IsCollection || !strings.EqualFold(dataType, "Number") && !strings.EqualFold(dataType, "Currency") {
		return storage.Value{}, false
	}
	value, assigned := frame.scalars[key]
	if !assigned || value.Kind == storage.ValueNull {
		if frame.assigned[key] {
			return storage.Value{}, false
		}
		return storage.NullValue(), true
	}
	switch value.Kind {
	case storage.ValueInteger:
		return value.Clone(), true
	case storage.ValueDecimal:
		if field.Type == storage.FieldDecimal {
			return value.Clone(), true
		}
		rational, ok := new(big.Rat).SetString(strings.TrimSpace(value.Decimal))
		if ok && rational.IsInt() && rational.Num().IsInt64() {
			return storage.IntegerValue(rational.Num().Int64()), true
		}
	}
	return storage.Value{}, false
}

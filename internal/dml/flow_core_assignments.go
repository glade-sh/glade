package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) flowCoreApplyDeclaredAssignment(assignment storage.FlowAssignment, operator string, frame *flowFrame) (bool, error) {
	key := flowFrameKey(assignment.Target)
	variable, declared := frame.variables[key]
	if !declared {
		return false, nil
	}
	if operator != "add" && operator != "subtract" {
		return false, nil
	}
	if variable.IsCollection {
		return false, nil
	}
	if strings.EqualFold(variable.DataType, "String") && operator == "add" {
		left, exists := frame.scalars[key]
		if !exists {
			left = storage.NullValue()
		}
		right, ok := e.flowStringAssignmentValue(assignment, frame)
		if !ok || (left.Kind != storage.ValueString && left.Kind != storage.ValueNull) ||
			(right.Kind != storage.ValueString && right.Kind != storage.ValueNull) {
			return false, nil
		}
		frame.scalars[key] = storage.StringValue(workflowValueString(left) + workflowValueString(right))
		frame.assigned[key] = true
		return true, nil
	}
	if !strings.EqualFold(variable.DataType, "Number") && !strings.EqualFold(variable.DataType, "Currency") {
		return false, nil
	}
	left, right := frame.scalars[key], storage.Value{}
	var ok bool
	right, ok = e.flowFrameValue(assignment, frame)
	if !ok || (left.Kind != storage.ValueInteger && left.Kind != storage.ValueDecimal) || (right.Kind != storage.ValueInteger && right.Kind != storage.ValueDecimal) {
		return false, nil
	}
	symbol := "+"
	if operator == "subtract" {
		symbol = "-"
	}
	sum, ok := preciseFormulaNumberBinary(symbol, formulaStorageValue(left), formulaStorageValue(right))
	if !ok {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s has unsupported numeric operands", assignment.Name)
	}
	value, ok := flowFrameFormulaValue(sum.asString(), frame)
	if !ok {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s has unsupported numeric result", assignment.Name)
	}
	frame.scalars[key] = value
	frame.assigned[key] = true
	return true, nil
}

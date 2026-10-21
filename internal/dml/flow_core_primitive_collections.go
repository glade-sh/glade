package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowCorePrimitiveCollection(variable storage.FlowVariable) bool {
	return variable.IsCollection && (strings.EqualFold(variable.DataType, "String") || strings.EqualFold(variable.DataType, "Number"))
}

func (e *Engine) flowCoreApplyPrimitiveAssignment(assignment storage.FlowAssignment, operator string, frame *flowFrame) (bool, error) {
	key := flowFrameKey(assignment.Target)
	variable, declared := frame.variables[key]
	if !declared {
		return false, nil
	}
	if e.InvocableFlow && !variable.IsCollection && strings.EqualFold(variable.DataType, "Number") && operator == "add" {
		left := frame.scalars[key]
		right, ok := e.flowFrameValue(assignment, frame)
		if !ok {
			return false, nil
		}
		value, _, ok := evaluateFlowFormulaValue("leftOperand + rightOperand", storage.Record{Fields: map[string]storage.Value{"leftOperand": left, "rightOperand": right}})
		if !ok {
			return false, nil
		}
		frame.scalars[key] = value
		frame.assigned[key] = true
		return true, nil
	}
	if variable.IsCollection {
		if operator != "add" || !flowCorePrimitiveCollection(variable) {
			return false, nil
		}
		item, ok := e.flowStringAssignmentValue(assignment, frame)
		if !ok || strings.EqualFold(variable.DataType, "String") && item.Kind != storage.ValueString && item.Kind != storage.ValueNull || strings.EqualFold(variable.DataType, "Number") && item.Kind != storage.ValueInteger && item.Kind != storage.ValueDecimal {
			return false, nil
		}
		collection := frame.scalars[key]
		if collection.Kind != storage.ValueList {
			collection = storage.ListValue()
		}
		collection.List = append(collection.List, item.Clone())
		frame.scalars[key] = collection
		frame.assigned[key] = true
		return true, nil
	}
	return false, nil
}

func (e *Engine) flowCorePrimitiveLoop(flowName, objectName string, record *storage.Record, definition storage.ObjectDefinition, loop storage.FlowLoop, frame *flowFrame) (bool, bool, error) {
	variable, declared := frame.variables[flowFrameKey(loop.CollectionReference)]
	if !declared || !flowCorePrimitiveCollection(variable) || !strings.EqualFold(loop.IterationOrder, "Asc") {
		return false, false, nil
	}
	collection := frame.scalars[flowFrameKey(loop.CollectionReference)]
	if collection.Kind != storage.ValueList {
		return false, false, nil
	}
	currentItem := strings.TrimSpace(loop.CurrentItemReference)
	if currentItem == "" {
		currentItem = loop.Name
	}
	key := flowFrameKey(currentItem)
	previous, existed := frame.scalars[key]
	defer func() {
		if existed {
			frame.scalars[key] = previous
		} else {
			delete(frame.scalars, key)
		}
	}()
	changed := false
	for _, item := range collection.List {
		frame.scalars[key] = item.Clone()
		stepChanged, err := e.applyFlowLoopBody(flowName, objectName, record, definition, loop.Steps, frame)
		if err != nil {
			return true, changed, err
		}
		changed = changed || stepChanged
	}
	return true, changed, nil
}
